package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"buf.build/go/protovalidate"
	protovalidatemw "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/protovalidate"
	chunkv1alpha1 "github.com/spacechunks/explorer/api/chunk/v1alpha1"
	instancev1alpha1 "github.com/spacechunks/explorer/api/instance/v1alpha1"
	mmv1alpha1 "github.com/spacechunks/matchmaking/api/v1alpha1"
	"github.com/spacechunks/matchmaking/internal/matchmaking"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Server struct {
	mmv1alpha1.UnimplementedMatchmakingServiceServer
	logger  *slog.Logger
	cfg     Config
	tickets *matchmaking.Store[matchmaking.Ticket]
}

func New(logger *slog.Logger, config Config, tickets *matchmaking.Store[matchmaking.Ticket]) *Server {
	return &Server{
		logger:  logger,
		cfg:     config,
		tickets: tickets,
	}
}

type Config struct {
	ListeAddr                            string
	ControlPlaneAddr                     string
	MatchInterval                        time.Duration
	AllocateInstanceForPendingMatchAfter time.Duration
	RemoveInactiveTicketsAfter           time.Duration
}

func (s Server) Run(ctx context.Context) error {
	validator, err := protovalidate.New()
	if err != nil {
		return fmt.Errorf("create validator: %w", err)
	}

	grpcServer := grpc.NewServer(
		grpc.Creds(insecure.NewCredentials()),
		grpc.ChainUnaryInterceptor(
			protovalidatemw.UnaryServerInterceptor(validator),
		),
	)

	conn, err := grpc.NewClient(s.cfg.ControlPlaneAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("create grpc client: %w", err)
	}

	mm := matchmaking.NewFlavorMatchMaker(
		s.logger.With("component", "matchmaker"),
		s.cfg.MatchInterval,
		s.cfg.AllocateInstanceForPendingMatchAfter,
		s.cfg.RemoveInactiveTicketsAfter,
		s.tickets,
		chunkv1alpha1.NewChunkServiceClient(conn),
		instancev1alpha1.NewInstanceServiceClient(conn),
	)

	go mm.Start(ctx)

	mmv1alpha1.RegisterMatchmakingServiceServer(grpcServer, s)

	lis, err := net.Listen("tcp", s.cfg.ListeAddr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	cancelCtx, cancel := context.WithCancel(ctx)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			s.logger.ErrorContext(ctx, "failed to start grpc server", "error", err)
			cancel()
		}
	}()

	<-cancelCtx.Done()
	grpcServer.GracefulStop()
	mm.Stop()
	return nil
}
