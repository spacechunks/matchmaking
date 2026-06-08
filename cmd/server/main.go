package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spacechunks/matchmaking/internal/matchmaking"
	"github.com/spacechunks/matchmaking/internal/server"
)

func main() {
	var (
		ctx, cancel = context.WithCancel(context.Background())
		logger      = slog.New(slog.NewJSONHandler(os.Stdout, nil))
		tickets     = matchmaking.NewStore[matchmaking.Ticket]()
		mm          = matchmaking.NewFlavorMatchMaker(
			logger.With("component", "matchmaker"),
			1*time.Second,
			tickets,
		)
		serv = server.New(logger, ":6789", tickets)
	)

	go mm.Start(ctx)
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
