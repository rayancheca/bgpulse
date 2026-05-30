// Command bgpulse is the BGPulse backend: it ingests BGP updates (synthetic in demo
// mode, or replayed from an MRT dump), classifies route leaks and prefix hijacks,
// maintains a live AS topology, and serves it over REST + WebSocket.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/rayancheca/bgpulse/backend/internal/config"
	"github.com/rayancheca/bgpulse/backend/internal/server"
)

func main() {
	cfg, err := config.Load(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "bgpulse: "+err.Error())
		os.Exit(2)
	}
	log := config.NewLogger(cfg)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app, err := server.New(cfg, log)
	if err != nil {
		log.Error("startup failed", "err", err)
		os.Exit(1)
	}
	if err := app.Run(ctx); err != nil {
		log.Error("runtime error", "err", err)
		os.Exit(1)
	}
	log.Info("clean shutdown")
}
