package topology

import (
	"sort"
	"time"

	"github.com/rayancheca/bgpulse/backend/internal/bgp"
)

// topOriginsLimit bounds how many origin ASes the stats frame carries for the sidebar.
const topOriginsLimit = 40

// NodeView is an immutable snapshot of an AS node.
type NodeView struct {
	ASN         uint32
	Name        string
	PrefixCount int
	RPKI        RPKICounts
	FirstSeen   time.Time
	LastSeen    time.Time
}

// EdgeView is an immutable snapshot of a directed edge.
type EdgeView struct {
	From, To    uint32
	Status      bgp.VFStatus
	Rel         bgp.RelStatus
	Count       int
	LeakCount   int
	HijackCount int
	LastEvent   time.Time
	LastEventID bgp.EventID
}

// SnapshotView is the full graph at a point in time, node- and edge-sorted.
type SnapshotView struct {
	Nodes       []NodeView
	Edges       []EdgeView
	GeneratedAt time.Time
}

// OriginStatView feeds one RPKI sidebar card.
type OriginStatView struct {
	ASN         uint32
	Name        string
	PrefixCount int
	RPKI        bgp.RPKIStatus
	Valid       int
	Invalid     int
	NotFound    int
	Throughput  []int
}

// StatsView is the running counters plus the top origin ASes.
type StatsView struct {
	TotalEvents  int64
	Announces    int64
	Withdraws    int64
	Leaks        int64
	Hijacks      int64
	RPKIValid    int64
	RPKIInvalid  int64
	RPKINotFound int64
	NodeCount    int
	EdgeCount    int
	EventsPerSec float64
	TopOrigins   []OriginStatView
}

// NeighborView describes one adjacency from an AS's perspective.
type NeighborView struct {
	ASN       uint32
	Direction string // "upstream" | "downstream"
	Rel       bgp.RelStatus
	Status    bgp.VFStatus
}

// ASNDetailView is the detail for one AS.
type ASNDetailView struct {
	Found     bool
	Node      NodeView
	Neighbors []NeighborView
	Prefixes  []string
	Sparkline []int
}

// EdgeDetailView is the detail (and raw events) for one directed edge.
type EdgeDetailView struct {
	Found  bool
	Edge   EdgeView
	Events []bgp.ClassifiedEvent
}

// FullSnapshot is the bundle sent to a client on connect.
type FullSnapshot struct {
	Topology SnapshotView
	Events   []bgp.ClassifiedEvent
	Stats    StatsView
}

func (n *ASNode) view() NodeView {
	return NodeView{
		ASN: n.ASN, Name: n.Name, PrefixCount: n.PrefixCount(), RPKI: n.RPKI,
		FirstSeen: n.FirstSeen, LastSeen: n.LastSeen,
	}
}

func (e *Edge) view() EdgeView {
	return EdgeView{
		From: e.Key.From, To: e.Key.To, Status: e.Status, Rel: e.Rel,
		Count: e.Count, LeakCount: e.LeakCount, HijackCount: e.HijackCount,
		LastEvent: e.LastEvent, LastEventID: e.LastEventID,
	}
}

func (a *Aggregator) buildTopology(now time.Time) SnapshotView {
	nodes := make([]NodeView, 0, len(a.graph.Nodes))
	for _, n := range a.graph.Nodes {
		nodes = append(nodes, n.view())
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ASN < nodes[j].ASN })

	edges := make([]EdgeView, 0, len(a.graph.Edges))
	for _, e := range a.graph.Edges {
		edges = append(edges, e.view())
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		return edges[i].To < edges[j].To
	})
	return SnapshotView{Nodes: nodes, Edges: edges, GeneratedAt: now}
}

func (a *Aggregator) buildStats() StatsView {
	s := a.stats
	v := StatsView{
		TotalEvents: s.TotalEvents, Announces: s.Announces, Withdraws: s.Withdraws,
		Leaks: s.Leaks, Hijacks: s.Hijacks, RPKIValid: s.RPKIValid,
		RPKIInvalid: s.RPKIInvalid, RPKINotFound: s.RPKINotFound,
		NodeCount: len(a.graph.Nodes), EdgeCount: len(a.graph.Edges),
		EventsPerSec: s.eventsPerSec,
	}
	v.TopOrigins = a.topOrigins()
	return v
}

func (a *Aggregator) topOrigins() []OriginStatView {
	origins := make([]*ASNode, 0)
	for _, n := range a.graph.Nodes {
		if n.Events > 0 {
			origins = append(origins, n)
		}
	}
	sort.Slice(origins, func(i, j int) bool {
		if origins[i].Events != origins[j].Events {
			return origins[i].Events > origins[j].Events
		}
		return origins[i].ASN < origins[j].ASN
	})
	if len(origins) > topOriginsLimit {
		origins = origins[:topOriginsLimit]
	}
	out := make([]OriginStatView, 0, len(origins))
	for _, n := range origins {
		out = append(out, OriginStatView{
			ASN: n.ASN, Name: n.Name, PrefixCount: n.PrefixCount(), RPKI: n.worstRPKI,
			Valid: n.RPKI.Valid, Invalid: n.RPKI.Invalid, NotFound: n.RPKI.NotFound,
			Throughput: n.series.series(),
		})
	}
	return out
}

func (a *Aggregator) buildASNDetail(asn uint32) ASNDetailView {
	n, ok := a.graph.Nodes[asn]
	if !ok {
		return ASNDetailView{Found: false}
	}
	var neighbors []NeighborView
	for key, e := range a.graph.Edges {
		switch asn {
		case key.From:
			neighbors = append(neighbors, NeighborView{ASN: key.To, Direction: "downstream", Rel: e.Rel, Status: e.Status})
		case key.To:
			neighbors = append(neighbors, NeighborView{ASN: key.From, Direction: "upstream", Rel: e.Rel.Invert(), Status: e.Status})
		}
	}
	sort.Slice(neighbors, func(i, j int) bool { return neighbors[i].ASN < neighbors[j].ASN })

	prefixes := make([]string, 0, len(n.prefixes))
	for p := range n.prefixes {
		prefixes = append(prefixes, p)
	}
	sort.Strings(prefixes)

	return ASNDetailView{
		Found: true, Node: n.view(), Neighbors: neighbors,
		Prefixes: prefixes, Sparkline: n.series.series(),
	}
}

func (a *Aggregator) buildEdgeDetail(from, to uint32, limit int) EdgeDetailView {
	e, ok := a.graph.Edges[EdgeKey{From: from, To: to}]
	if !ok {
		return EdgeDetailView{Found: false}
	}
	return EdgeDetailView{Found: true, Edge: e.view(), Events: a.ring.byEdge(from, to, limit)}
}
