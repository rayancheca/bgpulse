// Package topology maintains the live in-memory AS graph. A single goroutine (the
// Aggregator actor) exclusively owns the graph, the recent-event ring, and the
// stats, so there are no locks and no data races by construction: mutations arrive
// on the input channel and reads arrive as snapshot requests, both serialized by
// the actor's select loop. REST handlers receive immutable value snapshots; the
// WebSocket hub receives per-event broadcasts.
package topology

import (
	"time"

	"github.com/rayancheca/bgpulse/backend/internal/bgp"
)

// EdgeKey identifies a directed adjacency From->To. Edges are directed because
// relationship and valley-free semantics depend on direction.
type EdgeKey struct {
	From uint32
	To   uint32
}

// RPKICounts tallies origin-validation verdicts seen for an AS acting as origin.
type RPKICounts struct {
	Valid    int
	Invalid  int
	NotFound int
}

// ASNode is a vertex: one Autonomous System.
type ASNode struct {
	ASN         uint32
	Name        string
	prefixes    map[string]struct{} // CIDRs currently originated; drives PrefixCount
	RPKI        RPKICounts
	Events      int       // total events observed with this AS as origin (for ranking)
	worstRPKI   bgp.RPKIStatus
	series      *ringCounters // bounded throughput sparkline
	FirstSeen   time.Time
	LastSeen    time.Time
}

// PrefixCount returns the number of distinct prefixes currently originated.
func (n *ASNode) PrefixCount() int { return len(n.prefixes) }

// Edge is a directed adjacency observed on at least one AS_PATH.
type Edge struct {
	Key         EdgeKey
	Status      bgp.VFStatus  // worst status seen recently
	Rel         bgp.RelStatus // inferred relationship From->To
	Count       int
	LeakCount   int
	HijackCount int
	LastEvent   time.Time
	LastEventID bgp.EventID
	samples     []bgp.EventID // bounded FIFO of recent event ids traversing this edge
}

// ribEntry tracks the current announced state of one prefix (for origin-change
// detection and correct prefix-count maintenance on withdraw).
type ribEntry struct {
	originAS uint32
}

// TopologyGraph is the full in-memory AS graph, owned exclusively by the Aggregator.
type TopologyGraph struct {
	Nodes map[uint32]*ASNode
	Edges map[EdgeKey]*Edge
	rib   map[string]*ribEntry // CIDR -> current announced state
}

func newGraph() *TopologyGraph {
	return &TopologyGraph{
		Nodes: map[uint32]*ASNode{},
		Edges: map[EdgeKey]*Edge{},
		rib:   map[string]*ribEntry{},
	}
}

// node returns the node for asn, creating it on first sight.
func (g *TopologyGraph) node(asn uint32, now time.Time, name string) *ASNode {
	n, ok := g.Nodes[asn]
	if ok {
		return n
	}
	n = &ASNode{
		ASN:       asn,
		Name:      name,
		prefixes:  map[string]struct{}{},
		series:    newRingCounters(),
		FirstSeen: now,
		LastSeen:  now,
	}
	g.Nodes[asn] = n
	return n
}
