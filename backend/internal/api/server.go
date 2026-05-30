package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/rayancheca/bgpulse/backend/internal/bgp"
	"github.com/rayancheca/bgpulse/backend/internal/topology"
)

// Store is the read side the API depends on; *topology.Aggregator satisfies it.
type Store interface {
	Topology() topology.SnapshotView
	Events(limit int) []bgp.ClassifiedEvent
	Stats() topology.StatsView
	ASNDetail(asn uint32) topology.ASNDetailView
	EdgeDetail(from, to uint32, limit int) topology.EdgeDetailView
	Snapshot() topology.FullSnapshot
}

// HealthInfo is the static-ish health/source metadata reported at /api/health.
type HealthInfo struct {
	Mode         string
	Version      string
	BGPSource    string
	RelSource    string
	RPKISource   string
	LiveFellBack bool
}

// Server holds the REST handlers and their dependencies.
type Server struct {
	store      Store
	log        *slog.Logger
	health     HealthInfo
	corsOrigin string
	startedAt  time.Time
}

// NewServer builds an API server. corsOrigin "" means same-origin only.
func NewServer(store Store, log *slog.Logger, health HealthInfo, corsOrigin string, startedAt time.Time) *Server {
	return &Server{store: store, log: log, health: health, corsOrigin: corsOrigin, startedAt: startedAt}
}

// Routes returns the HTTP handler with REST routes, the optional WebSocket handler
// mounted at /ws, and the middleware chain applied.
func (s *Server) Routes(wsHandler http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/topology", s.handleTopology)
	mux.HandleFunc("GET /api/events", s.handleEvents)
	mux.HandleFunc("GET /api/stats", s.handleStats)
	mux.HandleFunc("GET /api/asn/{asn}", s.handleASN)
	mux.HandleFunc("GET /api/edge/{from}/{to}", s.handleEdge)
	if wsHandler != nil {
		mux.Handle("/ws", wsHandler)
	}
	return s.recoverer(s.requestLogger(s.cors(mux)))
}

// ---- middleware ----

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.log.Error("panic in handler", "err", rec, "path", r.URL.Path)
				writeErr(w, http.StatusInternalServerError, "internal error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.log.Debug("http", "method", r.Method, "path", r.URL.Path,
			"status", rec.status, "dur", time.Since(start).String())
	})
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.corsOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", s.corsOrigin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
