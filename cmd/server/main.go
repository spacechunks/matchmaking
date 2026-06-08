package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spacechunks/matchmaking/internal/matchmaking"
	"github.com/spacechunks/matchmaking/internal/server"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	tickets := matchmaking.NewStore[matchmaking.Ticket]()

	serv := server.New(logger, server.Config{}, tickets)

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		s := <-c
		logger.Info("received shutdown signal", "signal", s)
		cancel()
	}()

	if err := serv.Run(ctx); err != nil {
		logger.ErrorContext(ctx, "error running server", "err", err)
	}
}
