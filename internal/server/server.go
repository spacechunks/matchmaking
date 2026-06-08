package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"buf.build/go/protovalidate"
	protovalidatemw "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/protovalidate"
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

type Config struct {
	ListenAddr                              string
	AllocateInstanceForPendingMatchDuration time.Duration // TODO: better name
}

func New(logger *slog.Logger, config Config, tickets *matchmaking.Store[matchmaking.Ticket]) *Server {
	return &Server{
		logger:  logger,
		cfg:     config,
		tickets: tickets,
	}
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

	mmv1alpha1.RegisterMatchmakingServiceServer(grpcServer, s)

	lis, err := net.Listen("tcp", s.cfg.ListenAddr)
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
	return nil
}
