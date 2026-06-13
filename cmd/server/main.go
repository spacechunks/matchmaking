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
	viper.SetDefault("control_plane_endpoint", "localhost:9012")
	viper.SetDefault("control_plane_tls_enabled", true)
	viper.SetDefault("control_plane_api_token", "")
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
