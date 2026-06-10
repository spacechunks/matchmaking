package matchmaking

import (
	"context"
	"log/slog"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/spacechunks/matchmaking/internal/gameserver"
)

type MatchMaker interface {
}

type FlavorMatchMaker struct {
	logger *slog.Logger
	ticker *time.Ticker

	tickets *Store[Ticket]
	matches *Store[Match]
	alloc   gameserver.Allocator

	ticketPools    map[string]TicketPool
	pendingMatches map[string][]string

	allocInstanceForPendingMatchAfter time.Duration
}

func NewFlavorMatchMaker(
	logger *slog.Logger,
	matchEvalInterval time.Duration,
	allocInstanceForPendingMatchAfter time.Duration,
	tickets *Store[Ticket],
) *FlavorMatchMaker {
	return &FlavorMatchMaker{
		logger:                            logger,
		ticker:                            time.NewTicker(matchEvalInterval),
		tickets:                           tickets,
		ticketPools:                       make(map[string]TicketPool),
		matches:                           NewStore[Match](),
		pendingMatches:                    make(map[string][]string),
		alloc:                             &gameserver.MockAlloc{},
		allocInstanceForPendingMatchAfter: allocInstanceForPendingMatchAfter,
	}
}

func (m FlavorMatchMaker) Start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.ticker.C:
			// TODO: fetch latest flavor version and flavor (for min/max player counts)
			// let the previous matched players use the last flavor version

			for flavor, pool := range m.ticketPools {
				m.match(flavor, pool)
			}

			for _, match := range m.matches.View() {
				m.logger.Info("match", "match_id", match.ID, "flavor", match.FlavorID, "tickets", len(match.Tickets))
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
						t.MatchID = nil // make ticket be picked up by the pool again
						m.tickets.Update(t)
					}

					m.logger.Info("match has invalidated tickets, removing match", "match_id", match)
					m.matches.Delete(match.ID)
					continue
				}

				// all tickets in our match are still valid

				if match.Full {
					m.logger.Info(
						"creating instance and assignment",
						"match_id", match.ID,
					)

					if err := m.AllocateInstanceAndAssign(ctx, match); err != nil {
						m.logger.ErrorContext(
							ctx,
							"failed to allocate instance",
							"match_id", match.ID,
							"err", err,
						)
					}

					continue
				}

				if time.Now().After(match.CreatedAt.Add(m.allocInstanceForPendingMatchAfter)) {
					m.logger.Info("pending match created")
					if err := m.AllocateInstanceAndAssign(ctx, match); err != nil {
						m.logger.ErrorContext(
							ctx,
							"failed to allocate instance",
							"match_id", match.ID,
							"err", err,
						)
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

			// we delete and recreate all ticket pools in order to add new tickets
			// and put tickets from invalidated matches back into their pools.
			// this also has the benefit, that we will clean the map from flavors that
			// do not have tickets anymore.

			for k := range m.ticketPools {
				delete(m.ticketPools, k)
			}

			for _, t := range m.tickets.View() {
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

func (m FlavorMatchMaker) match(flavor string, pool TicketPool) {
	if len(pool.Tickets()) == 0 {
		return
	}

	// forget above
	// deal with this on the client side => if player wants  to join party => leave queue, remove ticket from pool
	// if instance does not start => put tickets into pool
	// if players leave or join while instance creation => invalidate match and put tickets into pool again
	// whats the worst thing that could happen with this approach => player leaves/joins party during server creation
	// => match gets invalidated all tickets, go back into pool, server will still be created and is a zombie.
	logger := m.logger.With("flavor_id", flavor)

	pending := m.pendingMatches[flavor]
	for _, matchID := range pending {
		match := m.matches.Get(matchID)
		if match == nil {
			m.logger.Warn("pending match not found", "match_id", matchID)
			continue // should not happen
		}

		matched := pool.FindTickets(10 - match.PlayerCount())

		for _, t := range matched {
			t.MatchID = &match.ID
			m.tickets.Update(t)
		}

		pool.RemoveAll(matched)
		match.Tickets = append(match.Tickets, matched...)

		if match.PlayerCount() == 10 {
			match.Full = true
			slices.DeleteFunc(m.pendingMatches[flavor], func(s string) bool {
				return s == match.ID
			})
		}

		m.matches.Update(*match)
	}

	matched := pool.FindTickets(10)

	if matched.PlayerCount() < 3 {
		logger.Info(
			"sum of matched players below minimum required",
			"match_len", matched.PlayerCount(),
		)
		return
	}

	defer pool.RemoveAll(matched)

	match := Match{
		ID:              uuid.NewString(),
		Tickets:         matched,
		FlavorID:        flavor,
		CreatedAt:       time.Now(),
		FlavorVersionID: "",
	}

	if match.PlayerCount() == 10 {
		match.Full = true
	}

	for _, t := range match.Tickets {
		t.MatchID = &match.ID
		m.tickets.Update(t)
	}

	m.matches.Add(match)
}

func (m FlavorMatchMaker) AllocateInstanceAndAssign(ctx context.Context, match Match) error {
	insID, err := m.alloc.AllocateInstance(ctx)
	if err != nil {
		return err
	}

	for _, t := range match.Tickets {
		t.Assignment = &Assignment{
			InstanceID: insID,
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
