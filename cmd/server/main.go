package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spacechunks/matchmaking/internal/matchmaking"
	"github.com/spacechunks/matchmaking/internal/server"
	"github.com/spf13/viper"
)

func main() {
	var (
		ctx, cancel = context.WithCancel(context.Background())
		logger      = slog.New(slog.NewJSONHandler(os.Stdout, nil))
		tickets     = matchmaking.NewStore[matchmaking.Ticket]()
	)

	viper.SetEnvPrefix("MM")
	viper.AutomaticEnv()
	viper.SetDefault("listen_addr", "0.0.0.0:6789")
	viper.SetDefault("control_plane_addr", "localhost:9012")
	viper.SetDefault("match_interval", "1s")
	viper.SetDefault("allocate_instance_for_pending_match_after", "15s")
	viper.SetDefault("remove_inactive_tickets_after", "1m")

	var cfg server.Config
	if err := viper.Unmarshal(&cfg); err != nil {
		logger.ErrorContext(ctx, "unable to decode config", "err", err)
		os.Exit(1)
	}

	serv := server.New(logger, cfg, tickets)

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		s := <-c
		logger.Info("received shutdown signal", "signal", s)
		cancel()
	}()

	if err := serv.Run(ctx); err != nil {
		logger.ErrorContext(ctx, "error running server", "err", err)
		os.Exit(1)
	}
}
