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
	"log/slog"
	"time"

	chunkv1alpha1 "github.com/spacechunks/explorer/api/chunk/v1alpha1"
)

type TicketStatus int

const (
	TicketStatusInactive = iota
	TicketStatusActive
	TicketStatusNoPlayableFlavorVersion
)

type Ticket struct {
	ID          string
	PlayerCount uint32
	Status      TicketStatus
	ChunkID     string
	FlavorID    string
	Assignment  *Assignment
	MatchID     *string
	CreatedAt   time.Time
}

func (t Ticket) GetID() string {
	return t.ID
}

type Assignment struct {
	InstanceID string
}

type Match struct {
	ID            string
	Tickets       TicketList
	Full          bool
	ChunkID       string
	FlavorID      string
	FlavorVersion *chunkv1alpha1.FlavorVersion
	CreatedAt     time.Time
}

func (m Match) GetID() string {
	return m.ID
}

func (m Match) PlayerCount() uint32 {
	return m.Tickets.PlayerCount()
}

type TicketList []Ticket

func (l TicketList) PlayerCount() uint32 {
	var sum uint32 = 0
	for _, ticket := range l {
		sum += ticket.PlayerCount
	}
	return sum
}

func (l TicketList) LogValue() slog.Value {
	ids := make([]string, 0, len(l))
	for _, t := range l {
		ids = append(ids, t.ID)
	}

	return slog.AnyValue(ids)
}
