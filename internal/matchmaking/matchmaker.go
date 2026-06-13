package matchmaking

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/google/uuid"
	chunkv1alpha1 "github.com/spacechunks/explorer/api/chunk/v1alpha1"
	instancev1alpha1 "github.com/spacechunks/explorer/api/instance/v1alpha1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	errFlavorVersionNotFound   = fmt.Errorf("flavor version not found")
	errNoFlavorVersionPlayable = fmt.Errorf("no flavor version playable")
)

type FlavorMatchMaker struct {
	logger *slog.Logger
	ticker *time.Ticker

	tickets *Store[Ticket]
	matches *Store[Match]

	ticketPools    map[string]TicketPool
	pendingMatches map[string][]string

	allocInstanceForPendingMatchAfter time.Duration
	removeInactiveTicketsAfter        time.Duration

	chunkClient chunkv1alpha1.ChunkServiceClient
	insClient   instancev1alpha1.InstanceServiceClient
}

func NewFlavorMatchMaker(
	logger *slog.Logger,
	matchEvalInterval time.Duration,
	allocInstanceForPendingMatchAfter time.Duration,
	removeInactiveTicketsAfter time.Duration,
	tickets *Store[Ticket],
	chunkClient chunkv1alpha1.ChunkServiceClient,
	insClient instancev1alpha1.InstanceServiceClient,
) *FlavorMatchMaker {
	return &FlavorMatchMaker{
		logger:                            logger,
		ticker:                            time.NewTicker(matchEvalInterval),
		tickets:                           tickets,
		ticketPools:                       make(map[string]TicketPool),
		matches:                           NewStore[Match](),
		pendingMatches:                    make(map[string][]string),
		allocInstanceForPendingMatchAfter: allocInstanceForPendingMatchAfter,
		removeInactiveTicketsAfter:        removeInactiveTicketsAfter,
		chunkClient:                       chunkClient,
		insClient:                         insClient,
	}
}

func (m FlavorMatchMaker) Start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.ticker.C:
			for flavorID, pool := range m.ticketPools {
				ver, err := m.findPlayableFlavorVersion(ctx, flavorID)
				if err != nil {
					m.logger.ErrorContext(ctx, "error finding playable flavor version", "err", err)
					if errors.Is(err, errNoFlavorVersionPlayable) || errors.Is(err, errFlavorVersionNotFound) {
						// tickets will be removed
						for _, t := range pool.Tickets() {
							m.logger.WarnContext(ctx, "ticket is non-playable", "ticket_id", t.ID, "err", err)
							t.Status = TicketStatusNoPlayableFlavorVersion
							m.tickets.Update(t)
						}
						continue
					}
				}

				m.generateMatches(flavorID, ver, pool)
			}

			m.checkAndDeployMatches(ctx)

			// we delete and recreate all ticket pools in order to add new tickets
			// and put tickets from invalidated matches back into their pools.
			// this also has the benefit, that we will clean the map from flavors that
			// do not have tickets anymore.

			for k := range m.ticketPools {
				delete(m.ticketPools, k)
			}

			for _, t := range m.tickets.View() {
				eligibleForRemoval := t.Assignment != nil ||
					t.Status == TicketStatusNoPlayableFlavorVersion ||
					t.Status == TicketStatusInactive

				// perform this check before creating the ticket pool, so we don't create pools
				// for flavors that potentially do not have any tickets.
				if eligibleForRemoval && time.Now().After(t.CreatedAt.Add(m.removeInactiveTicketsAfter)) {
					m.tickets.Delete(t.ID)
					continue
				}

				if _, ok := m.ticketPools[t.FlavorID]; !ok {
					m.ticketPools[t.FlavorID] = TicketPool{
						tickets: make(map[string]Ticket),
					}
				}

				m.ticketPools[t.FlavorID].Add(t) // the pool will ignore duplicates and tickets that already have assignments
			}
		}
	}
}

func (m FlavorMatchMaker) generateMatches(flavorID string, version *chunkv1alpha1.FlavorVersion, pool TicketPool) {
	if len(pool.Tickets()) == 0 {
		return
	}

	// forget above
	// deal with this on the client side => if player wants  to join party => leave queue, remove ticket from pool
	// if instance does not start => put tickets into pool
	// if players leave or join while instance creation => invalidate match and put tickets into pool again
	// whats the worst thing that could happen with this approach => player leaves/joins party during server creation
	// => match gets invalidated all tickets, go back into pool, server will still be created and is a zombie.
	logger := m.logger.With("flavor_id", flavorID)

	pending := m.pendingMatches[flavorID]
	for _, matchID := range pending {
		match := m.matches.Get(matchID)
		if match == nil {
			m.logger.Warn("pending match not found", "match_id", matchID)
			continue // should not happen
		}

		var (
			maxPlayers = match.FlavorVersion.MaxPlayers
			matched    = pool.FindTickets(maxPlayers - match.PlayerCount())
		)

		for _, t := range matched {
			t.MatchID = &match.ID
			m.tickets.Update(t)
		}

		pool.RemoveAll(matched)
		match.Tickets = append(match.Tickets, matched...)

		if match.PlayerCount() == maxPlayers {
			match.Full = true
			m.pendingMatches[flavorID] = slices.DeleteFunc(m.pendingMatches[flavorID], func(s string) bool {
				return s == match.ID
			})
		}

		m.matches.Update(*match)
	}

	matched := pool.FindTickets(10)

	if matched.PlayerCount() < version.MinPlayers {
		logger.Info(
			"sum of matched players below minimum required",
			"tickets", matched,
		)
		return
	}

	defer pool.RemoveAll(matched)

	match := Match{
		ID:        uuid.NewString(),
		Tickets:   matched,
		FlavorID:  flavorID,
		CreatedAt: time.Now(),

		// assigning the flavor version to the match and not to the tickets
		// ensures that if there happen to be min/max player changes while
		// matchmaking, the previously created match can still operate on
		// correct values.
		//
		// if we didn't store the flavor version, it could happen that while
		// searching for other players the min/max players requirements change.
		// players could end up in a match where they would exceed the max
		// player count or be too few to actually start the game.
		FlavorVersion: version,
	}

	if match.PlayerCount() == version.MaxPlayers {
		match.Full = true
	}

	for _, t := range match.Tickets {
		t.MatchID = &match.ID
		m.tickets.Update(t)
	}

	m.matches.Add(match)
}

func (m FlavorMatchMaker) checkAndDeployMatches(ctx context.Context) {
	for _, match := range m.matches.View() {
		logger := m.logger.With(
			"match_id", match.ID,
			"tickets", match.Tickets,
			"flavor_id", match.FlavorID,
			"flavor_version_id", match.FlavorVersion.Id,
		)

		logger.Info("found match")

		var (
			invalidated []Ticket
			valid       []Ticket
		)

		// check if any tickets have been invalidated as of now. do not continue processing.
		for _, t := range match.Tickets {
			if tmp := m.tickets.Get(t.ID); tmp == nil {
				m.logger.Info("invalidated ticket", "match_id", match.ID, "ticket", t.ID)
				invalidated = append(invalidated, t)
				continue
			}
			valid = append(valid, t)
		}

		if len(invalidated) > 0 {
			for _, t := range valid {
				t.MatchID = nil    // make ticket be picked up by the pool again
				t.Assignment = nil // clear the assignment if there is any
				m.tickets.Update(t)
			}

			logger.Info("match has invalidated tickets, removing match", "match_id", match)
			m.matches.Delete(match.ID)
			continue
		}

		// all tickets in our match are still valid

		if match.Full {
			logger.Info("creating instance and assignment")

			if err := m.AllocateInstanceAndAssign(ctx, match); err != nil {
				logger.ErrorContext(ctx, "failed to allocate instance", "err", err)
			}

			continue
		}

		if time.Now().After(match.CreatedAt.Add(m.allocInstanceForPendingMatchAfter)) {
			m.logger.Info("pending match created")
			if err := m.AllocateInstanceAndAssign(ctx, match); err != nil {
				logger.ErrorContext(ctx, "failed to allocate instance", "err", err)
			}
			continue
		}

		containsPending := slices.ContainsFunc(m.pendingMatches[match.FlavorID], func(s string) bool {
			return s == match.ID
		})

		if containsPending {
			continue
		}

		m.pendingMatches[match.FlavorID] = append(m.pendingMatches[match.FlavorID], match.ID)
	}
}

func (m FlavorMatchMaker) AllocateInstanceAndAssign(ctx context.Context, match Match) error {
	resp, err := m.insClient.RunFlavorVersion(ctx, &instancev1alpha1.RunFlavorVersionRequest{
		FlavorVersionId: match.FlavorVersion.Id,
		OrderedBy:       "", // provide none for now
	})
	if err != nil {
		return err
	}

	for _, t := range match.Tickets {
		t.Assignment = &Assignment{
			InstanceID: resp.Instance.Id,
		}
		m.tickets.Update(t)
	}

	// instance has been allocated and associated to the tickets, so the match is no longer needed
	m.pendingMatches[match.FlavorID] = slices.DeleteFunc(m.pendingMatches[match.FlavorID], func(matchID string) bool {
		return match.ID == matchID
	})

	m.matches.Delete(match.ID)
	return nil
}

func (m FlavorMatchMaker) Stop() {
	m.ticker.Stop()
}

func (m FlavorMatchMaker) findPlayableFlavorVersion(
	ctx context.Context,
	flavorID string,
) (*chunkv1alpha1.FlavorVersion, error) {
	resp, err := m.chunkClient.GetFlavor(ctx, &chunkv1alpha1.GetFlavorRequest{
		Id: flavorID,
	})

	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return nil, errFlavorVersionNotFound
		}
		return nil, fmt.Errorf("get flavor: %w", err)
	}

	for _, v := range resp.Flavor.Versions {
		if v.BuildStatus == chunkv1alpha1.BuildStatus_COMPLETED {
			return v, nil
		}
	}

	return nil, errNoFlavorVersionPlayable
}
