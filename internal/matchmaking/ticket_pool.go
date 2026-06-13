/*
 A basic matchmaking service for the Chunk Explorer.
 Copyright (C) 2026 Yannic Rieger <oss@76k.io>

 This program is free software: you can redistribute it and/or modify
 it under the terms of the GNU Affero General Public License as published by
 the Free Software Foundation, either version 3 of the License, or
 (at your option) any later version.

 This program is distributed in the hope that it will be useful,
 but WITHOUT ANY WARRANTY; without even the implied warranty of
 MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 GNU Affero General Public License for more details.

 You should have received a copy of the GNU Affero General Public License
 along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

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

	if ticket.Assignment != nil || ticket.MatchID != nil || ticket.Status != TicketStatusActive {
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
func (p TicketPool) FindTickets(max uint32) TicketList {
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
	for s := int(max); s >= 0; s-- {
		if dp[s] {
			best = s
			break
		}
	}

	// reconstruct which tickets were used
	for s := best; s > 0; {
		idx := from[s]
		chosen[idx] = true
		s -= int(ticketSlice[idx].PlayerCount)
	}

	var result []Ticket
	for i, t := range ticketSlice {
		if chosen[i] {
			result = append(result, t)
		}
	}

	return result
}
