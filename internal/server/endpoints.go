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
	t := s.tickets.Get(req.TicketId)
	if t == nil {
		return nil, status.Error(codes.NotFound, "ticket not found")
	}

	ret := &mmv1alpha1.GetTicketResponse{
		Ticket: &mmv1alpha1.Ticket{
			Id:          t.ID,
			FlavorId:    t.FlavorID,
			PlayerCount: t.PlayerCount,
			Status:      mmv1alpha1.TicketStatus(t.Status),
		},
	}

	if t.Assignment != nil {
		ret.Ticket.Assignment = &mmv1alpha1.Assignment{
			InstanceId: t.Assignment.InstanceID,
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
