package api

import (
	"net/http"
	"strconv"
	"time"
)

const (
	defaultEventsLimit = 200
	maxEventsLimit     = 1000
	defaultEdgeEvents  = 50
)

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeOK(w, HealthDTO{
		OK:        true,
		Mode:      s.health.Mode,
		Version:   s.health.Version,
		UptimeSec: int64(time.Since(s.startedAt).Seconds()),
		Sources: SourceHealthDTO{
			BGP:           s.health.BGPSource,
			Relationships: s.health.RelSource,
			RPKI:          s.health.RPKISource,
			LiveFellBack:  s.health.LiveFellBack,
		},
	})
}

func (s *Server) handleTopology(w http.ResponseWriter, _ *http.Request) {
	writeOK(w, topologyViewToDTO(s.store.Topology()))
}

func (s *Server) handleStats(w http.ResponseWriter, _ *http.Request) {
	writeOK(w, statsViewToDTO(s.store.Stats()))
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	limit, ok := parseLimit(r, defaultEventsLimit, maxEventsLimit)
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid limit")
		return
	}
	dtos := classifiedSliceToDTO(s.store.Events(limit))
	writeOK(w, EventsDTO{Events: dtos, Count: len(dtos), Limit: limit})
}

func (s *Server) handleASN(w http.ResponseWriter, r *http.Request) {
	asn, ok := parseASN(r.PathValue("asn"))
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid ASN")
		return
	}
	d := s.store.ASNDetail(asn)
	if !d.Found {
		writeErr(w, http.StatusNotFound, "unknown ASN")
		return
	}
	writeOK(w, asnDetailToDTO(d))
}

func (s *Server) handleEdge(w http.ResponseWriter, r *http.Request) {
	from, ok1 := parseASN(r.PathValue("from"))
	to, ok2 := parseASN(r.PathValue("to"))
	if !ok1 || !ok2 {
		writeErr(w, http.StatusBadRequest, "invalid ASN")
		return
	}
	limit, ok := parseLimit(r, defaultEdgeEvents, maxEventsLimit)
	if !ok {
		limit = defaultEdgeEvents
	}
	d := s.store.EdgeDetail(from, to, limit)
	if !d.Found {
		writeErr(w, http.StatusNotFound, "unknown edge")
		return
	}
	writeOK(w, edgeDetailToDTO(d))
}

// parseASN parses a uint32 ASN from a path value.
func parseASN(s string) (uint32, bool) {
	v, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, false
	}
	return uint32(v), true
}

// parseLimit reads ?limit=, defaulting when absent and clamping to [1,max]. It
// returns ok=false only when an explicitly provided limit is unparseable.
func parseLimit(r *http.Request, def, max int) (int, bool) {
	q := r.URL.Query().Get("limit")
	if q == "" {
		return def, true
	}
	v, err := strconv.Atoi(q)
	if err != nil {
		return 0, false
	}
	if v < 1 {
		v = 1
	}
	if v > max {
		v = max
	}
	return v, true
}
