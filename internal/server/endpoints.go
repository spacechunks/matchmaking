package server

import (
	"context"
	"fmt"

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

	return &mmv1alpha1.GetTicketResponse{
		Ticket: &mmv1alpha1.Ticket{
			Id:          t.ID,
			FlavorId:    t.FlavorID,
			PlayerCount: uint32(t.PlayerCount),
			Active:      t.Active,
		},
	}, nil
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
		PlayerCount: int(req.PlayerCount),
		FlavorID:    req.FlavorId,
	}

	s.tickets.Add(t)

	return &mmv1alpha1.CreateTicketResponse{
		Ticket: &mmv1alpha1.Ticket{
			Id:          t.ID,
			FlavorId:    t.FlavorID,
			PlayerCount: uint32(t.PlayerCount),
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

	t.Active = true
	s.tickets.Update(*t)

	return &mmv1alpha1.ActivateTicketResponse{}, nil
}
