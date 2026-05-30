// Package server is the composition root: it assembles the data sources, classifier,
// aggregator, hub, and HTTP server, and runs them under one errgroup with graceful
// shutdown.
package server

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"log/slog"

	"github.com/rayancheca/bgpulse/backend/internal/api"
	"github.com/rayancheca/bgpulse/backend/internal/bgp"
	"github.com/rayancheca/bgpulse/backend/internal/classify"
	"github.com/rayancheca/bgpulse/backend/internal/config"
	"github.com/rayancheca/bgpulse/backend/internal/pipeline"
	"github.com/rayancheca/bgpulse/backend/internal/topology"
	"github.com/rayancheca/bgpulse/backend/internal/wshub"
)

// Server wires and runs the whole backend.
type Server struct {
	cfg       config.Config
	log       *slog.Logger
	httpSrv   *http.Server
	agg       *topology.Aggregator
	hub       *wshub.Hub
	pipeline  *pipeline.Pipeline
	rtr       rtrRunner
	broadcast chan bgp.ClassifiedEvent
}

// rtrRunner is the subset of *rtr.Client the server drives (nil unless live RTR).
type rtrRunner interface {
	Run(ctx context.Context) error
}

// New assembles the server from validated config.
func New(cfg config.Config, log *slog.Logger) (*Server, error) {
	src, err := buildSources(cfg, log)
	if err != nil {
		return nil, err
	}
	log.Info("data sources", "mode", cfg.Mode, "bgp", src.health.BGPSource,
		"relationships", src.health.RelSource, "rpki", src.health.RPKISource)

	cls := classify.New(src.rel, src.validator)
	classified := make(chan bgp.ClassifiedEvent, config.ClassifiedBufSize)
	broadcast := make(chan bgp.ClassifiedEvent, config.BroadcastBufSize)

	agg := topology.NewAggregator(classified, broadcast, cfg.RingCapacity, src.rel)
	hub := wshub.New(func() []byte {
		b, err := api.SnapshotFrame(agg.Snapshot())
		if err != nil {
			log.Error("snapshot frame", "err", err)
			return nil
		}
		return b
	}, log)
	pl := pipeline.New(src.source, cls, classified, log)

	apiSrv := api.NewServer(agg, log, src.health, cfg.CORSOrigin, time.Now())
	var handler http.Handler = apiSrv.Routes(hub.Handler())
	if cfg.StaticDir != "" {
		handler = staticFallback(handler, cfg.StaticDir)
	}

	s := &Server{
		cfg: cfg, log: log,
		httpSrv:   &http.Server{Addr: cfg.ListenAddr, Handler: handler},
		agg:       agg,
		hub:       hub,
		pipeline:  pl,
		broadcast: broadcast,
	}
	if src.rtr != nil {
		s.rtr = src.rtr
	}
	return s, nil
}

// Run starts every stage and blocks until ctx is cancelled or a stage errors.
func (s *Server) Run(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error { s.agg.Run(ctx); return nil })
	g.Go(func() error { s.hub.Run(ctx); return nil })
	g.Go(func() error { return s.pipeline.Run(ctx) })
	g.Go(func() error { return s.forwardBroadcasts(ctx) })
	if s.rtr != nil {
		g.Go(func() error {
			if err := s.rtr.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
				return err
			}
			return nil
		})
	}
	g.Go(func() error { return s.serveHTTP(ctx) })

	// A signal-driven shutdown cancels the parent context; every stage then returns
	// context.Canceled, which is a clean stop, not a runtime error. A genuine
	// failure surfaces as a different (first) error and propagates.
	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}

func (s *Server) serveHTTP(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), config.ShutdownTimeout)
		defer cancel()
		_ = s.httpSrv.Shutdown(shutCtx)
	}()
	s.log.Info("listening", "addr", s.cfg.ListenAddr, "mode", s.cfg.Mode)
	if err := s.httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// forwardBroadcasts maps aggregator events and periodic stats into WebSocket frames.
func (s *Server) forwardBroadcasts(ctx context.Context) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var seq uint64
	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-s.broadcast:
			if !ok {
				return nil
			}
			seq++
			if b, err := api.EventFrame(seq, ev); err == nil {
				s.hub.Broadcast(b)
			}
		case <-ticker.C:
			seq++
			if b, err := api.StatsFrame(seq, s.agg.Stats()); err == nil {
				s.hub.Broadcast(b)
			}
		}
	}
}

// staticFallback serves a built single-page frontend from dir, routing /api and /ws
// to the API handler and falling back to index.html for client-side routes.
func staticFallback(apiHandler http.Handler, dir string) http.Handler {
	fileServer := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api") || r.URL.Path == "/ws" {
			apiHandler.ServeHTTP(w, r)
			return
		}
		clean := filepath.Join(dir, filepath.Clean("/"+r.URL.Path))
		if st, err := os.Stat(clean); err == nil && !st.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(dir, "index.html"))
	})
}
