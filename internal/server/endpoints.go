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

package server

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	mmv1alpha1 "github.com/spacechunks/matchmaking/api/v1alpha1"
	"github.com/spacechunks/matchmaking/internal/matchmaking"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s Server) GetTicket(
	_ context.Context,
	req *mmv1alpha1.GetTicketRequest,
) (*mmv1alpha1.GetTicketResponse, error) {
	ticket := s.tickets.Get(req.TicketId)
	if ticket == nil {
		return nil, status.Error(codes.NotFound, "ticket not found")
	}

	ret := &mmv1alpha1.GetTicketResponse{
		Ticket: &mmv1alpha1.Ticket{
			Id:          ticket.ID,
			FlavorId:    ticket.FlavorID,
			PlayerCount: ticket.PlayerCount,
			Status:      mmv1alpha1.TicketStatus(ticket.Status),
		},
	}

	if ticket.Assignment != nil {
		ret.Ticket.Assignment = &mmv1alpha1.Assignment{
			InstanceId: ticket.Assignment.InstanceID,
		}
	}

	if ticket.MatchID != nil {
		if m := s.matches.Get(*ticket.MatchID); m != nil {
			ret.Ticket.Match = &mmv1alpha1.Match{
				Id:          m.ID,
				PlayerCount: m.PlayerCount(),
			}
		}
	}

	return ret, nil
}

func (s Server) RemoveTicket(
	_ context.Context,
	req *mmv1alpha1.RemoveTicketRequest,
) (*mmv1alpha1.RemoveTicketResponse, error) {
	s.tickets.Delete(req.TicketId)
	return &mmv1alpha1.RemoveTicketResponse{}, nil
}

func (s Server) RemoveAllTickets(
	_ context.Context,
	_ *mmv1alpha1.RemoveAllTicketsRequest,
) (*mmv1alpha1.RemoveAllTicketsResponse, error) {
	s.tickets.Clear()
	return &mmv1alpha1.RemoveAllTicketsResponse{}, nil
}

func (s Server) CreateTicket(
	_ context.Context,
	req *mmv1alpha1.CreateTicketRequest,
) (*mmv1alpha1.CreateTicketResponse, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate id: %w", err)
	}

	t := matchmaking.Ticket{
		ID:          id.String(),
		PlayerCount: req.PlayerCount,
		FlavorID:    req.FlavorId,
		CreatedAt:   time.Now(),
	}

	s.tickets.Add(t)

	return &mmv1alpha1.CreateTicketResponse{
		Ticket: &mmv1alpha1.Ticket{
			Id:          t.ID,
			FlavorId:    t.FlavorID,
			PlayerCount: t.PlayerCount,
			Status:      matchmaking.TicketStatusInactive,
		},
	}, nil
}

func (s Server) ActivateTicket(
	_ context.Context,
	req *mmv1alpha1.ActivateTicketRequest,
) (*mmv1alpha1.ActivateTicketResponse, error) {
	t := s.tickets.Get(req.TicketId)
	if t == nil {
		return nil, status.Error(codes.NotFound, "ticket not found")
	}

	// you should not be able to set the ticket status to active
	// if the status is NO_PLAYABLE_FLAVOR_VERSION. so only inactive
	// tickets can be activated.
	if t.Status != matchmaking.TicketStatusInactive {
		return nil, status.Error(codes.FailedPrecondition, "ticket is already active or has been invalidated")
	}

	t.Status = matchmaking.TicketStatusActive
	s.tickets.Update(*t)

	return &mmv1alpha1.ActivateTicketResponse{}, nil
}
