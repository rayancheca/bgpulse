package topology

import (
	"context"
	"time"

	"github.com/rayancheca/bgpulse/backend/internal/bgp"
)

const (
	snapshotEventsLimit = 200 // events bundled into the on-connect snapshot
	rateTickInterval    = time.Second
)

// NameLookup resolves an ASN to a human-readable name. *relationships.RelStore
// satisfies it; nil yields empty names.
type NameLookup interface {
	Name(asn uint32) string
}

type snapKind int

const (
	snapTopology snapKind = iota
	snapEvents
	snapStats
	snapASN
	snapEdge
	snapFull
)

type snapReq struct {
	kind  snapKind
	asn   uint32
	from  uint32
	to    uint32
	limit int
	reply chan any
}

// Aggregator is the single-writer actor that owns the graph, ring, and stats.
type Aggregator struct {
	in       <-chan bgp.ClassifiedEvent
	out      chan<- bgp.ClassifiedEvent
	snapReqs chan snapReq
	done     chan struct{}

	graph *TopologyGraph
	ring  *EventRing
	stats Stats
	names NameLookup
}

// NewAggregator builds an aggregator. in delivers classified events; out (optional)
// receives a non-blocking broadcast copy of each event for the WebSocket hub.
func NewAggregator(in <-chan bgp.ClassifiedEvent, out chan<- bgp.ClassifiedEvent, ringCap int, names NameLookup) *Aggregator {
	return &Aggregator{
		in:       in,
		out:      out,
		snapReqs: make(chan snapReq),
		done:     make(chan struct{}),
		graph:    newGraph(),
		ring:     newEventRing(ringCap),
		names:    names,
	}
}

func (a *Aggregator) name(asn uint32) string {
	if a.names == nil {
		return ""
	}
	return a.names.Name(asn)
}

// Run is the single writer. It returns when in is closed or ctx is cancelled.
func (a *Aggregator) Run(ctx context.Context) {
	defer close(a.done)
	ticker := time.NewTicker(rateTickInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-a.in:
			if !ok {
				return
			}
			a.apply(ev)
			a.broadcast(ev)
		case req := <-a.snapReqs:
			req.reply <- a.handle(req)
		case <-ticker.C:
			a.stats.tick()
		}
	}
}

// broadcast forwards an event to the hub without ever blocking the writer.
func (a *Aggregator) broadcast(ev bgp.ClassifiedEvent) {
	if a.out == nil {
		return
	}
	select {
	case a.out <- ev:
	default: // hub is slow; drop the broadcast (clients re-sync via snapshot)
	}
}

func (a *Aggregator) handle(req snapReq) any {
	switch req.kind {
	case snapTopology:
		return a.buildTopology(time.Now())
	case snapEvents:
		return a.ring.recent(req.limit)
	case snapStats:
		return a.buildStats()
	case snapASN:
		return a.buildASNDetail(req.asn)
	case snapEdge:
		return a.buildEdgeDetail(req.from, req.to, req.limit)
	case snapFull:
		return FullSnapshot{
			Topology: a.buildTopology(time.Now()),
			Events:   a.ring.recent(snapshotEventsLimit),
			Stats:    a.buildStats(),
		}
	default:
		return nil
	}
}

// request sends a snapshot request to the writer and returns its reply, or nil if
// the aggregator has stopped.
func (a *Aggregator) request(req snapReq) any {
	req.reply = make(chan any, 1)
	select {
	case a.snapReqs <- req:
	case <-a.done:
		return nil
	}
	select {
	case r := <-req.reply:
		return r
	case <-a.done:
		return nil
	}
}

// Topology returns a consistent graph snapshot.
func (a *Aggregator) Topology() SnapshotView {
	if v, ok := a.request(snapReq{kind: snapTopology}).(SnapshotView); ok {
		return v
	}
	return SnapshotView{}
}

// Events returns up to limit recent events, newest first.
func (a *Aggregator) Events(limit int) []bgp.ClassifiedEvent {
	if v, ok := a.request(snapReq{kind: snapEvents, limit: limit}).([]bgp.ClassifiedEvent); ok {
		return v
	}
	return nil
}

// Stats returns the running counters and top origins.
func (a *Aggregator) Stats() StatsView {
	if v, ok := a.request(snapReq{kind: snapStats}).(StatsView); ok {
		return v
	}
	return StatsView{}
}

// ASNDetail returns the detail for one AS (Found=false if unknown).
func (a *Aggregator) ASNDetail(asn uint32) ASNDetailView {
	if v, ok := a.request(snapReq{kind: snapASN, asn: asn}).(ASNDetailView); ok {
		return v
	}
	return ASNDetailView{}
}

// EdgeDetail returns the detail and recent events for one directed edge.
func (a *Aggregator) EdgeDetail(from, to uint32, limit int) EdgeDetailView {
	if v, ok := a.request(snapReq{kind: snapEdge, from: from, to: to, limit: limit}).(EdgeDetailView); ok {
		return v
	}
	return EdgeDetailView{}
}

// Snapshot returns the full bundle sent to a client on connect.
func (a *Aggregator) Snapshot() FullSnapshot {
	if v, ok := a.request(snapReq{kind: snapFull}).(FullSnapshot); ok {
		return v
	}
	return FullSnapshot{}
}
