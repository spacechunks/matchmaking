package matchmaking

import (
	"maps"
	"slices"
)

type TicketPool struct {
	tickets map[string]Ticket
}

func (p TicketPool) Add(ticket Ticket) {
	if _, ok := p.tickets[ticket.ID]; ok {
		return
	}

	if ticket.Assignment != nil || ticket.MatchID != nil || !ticket.Active {
		return
	}

	p.tickets[ticket.ID] = ticket
}

func (p TicketPool) RemoveAll(remove []Ticket) {
	for _, t := range remove {
		delete(p.tickets, t.ID)
	}
}

func (p TicketPool) Tickets() []Ticket {
	return slices.Collect(maps.Values(p.tickets))
}

// FindTickets returns the first combination of tickets that sums to exactly `target`. Cooked by AI.
func (p TicketPool) FindTickets(max int) TicketList {
	n := len(p.tickets)
	if n == 0 {
		return nil
	}

	// dp[i] = true if sum i is achievable
	dp := make([]bool, max+1)
	dp[0] = true

	// track which ticket was used to reach each sum
	from := make([]int, max+1) // from[i] = ticket index that completed sum i
	for i := range from {
		from[i] = -1
	}

	chosen := make([]bool, n) // chosen[i] = ticket i was used

	ticketSlice := slices.Collect(maps.Values(p.tickets))

	for i, t := range ticketSlice {
		// iterate backwards to avoid reusing the same ticket
		for s := max; s >= t.PlayerCount; s-- {
			if dp[s-t.PlayerCount] && !dp[s] {
				dp[s] = true
				from[s] = i
			}
		}
	}

	// find the best (highest) achievable sum
	best := 0
	for s := max; s >= 0; s-- {
		if dp[s] {
			best = s
			break
		}
	}

	// reconstruct which tickets were used
	for s := best; s > 0; {
		idx := from[s]
		chosen[idx] = true
		s -= ticketSlice[idx].PlayerCount
	}

	var result []Ticket
	for i, t := range ticketSlice {
		if chosen[i] {
			result = append(result, t)
		}
	}

	return result
}
