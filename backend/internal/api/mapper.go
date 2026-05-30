package api

import (
	"net/netip"
	"time"

	"github.com/rayancheca/bgpulse/backend/internal/bgp"
	"github.com/rayancheca/bgpulse/backend/internal/topology"
)

// rfc3339 renders a timestamp as an RFC3339 UTC string ("" for the zero time).
func rfc3339(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func addrString(a netip.Addr) string {
	if !a.IsValid() {
		return ""
	}
	return a.String()
}

// classifiedToDTO maps a classified event to its wire form.
func classifiedToDTO(ev bgp.ClassifiedEvent) ClassifiedDTO {
	e := ev.Event
	comms := make([]CommunityDTO, 0, len(e.Communities))
	for _, c := range e.Communities {
		comms = append(comms, CommunityDTO{ASN: c.ASN, Value: c.Value})
	}
	hops := make([]PathHopDTO, 0, len(ev.Hops))
	for _, h := range ev.Hops {
		hops = append(hops, PathHopDTO{From: h.From, To: h.To, Rel: h.Rel.String(), IsOffender: h.IsOffender})
	}
	path := e.ASPath
	if path == nil {
		path = []uint32{}
	}
	return ClassifiedDTO{
		ID: string(e.ID), Seq: e.Seq, Timestamp: rfc3339(e.Timestamp), Kind: e.Kind.String(),
		Prefix: e.Prefix.String(), PeerAs: e.PeerAS, ASPath: path, NextHop: addrString(e.NextHop),
		Communities: comms, OriginAs: e.OriginAS, VfStatus: ev.VFStatus.String(),
		RpkiStatus: ev.RPKIStatus.String(), Hops: hops, OffenderAs: ev.OffenderAS, Reason: ev.Reason,
	}
}

func classifiedSliceToDTO(evs []bgp.ClassifiedEvent) []ClassifiedDTO {
	out := make([]ClassifiedDTO, 0, len(evs))
	for _, ev := range evs {
		out = append(out, classifiedToDTO(ev))
	}
	return out
}

func nodeViewToDTO(n topology.NodeView) NodeDTO {
	return NodeDTO{
		ASN: n.ASN, Name: n.Name, PrefixCount: n.PrefixCount,
		RPKI:      RPKICountsDTO{Valid: n.RPKI.Valid, Invalid: n.RPKI.Invalid, NotFound: n.RPKI.NotFound},
		FirstSeen: rfc3339(n.FirstSeen), LastSeen: rfc3339(n.LastSeen),
	}
}

func edgeViewToDTO(e topology.EdgeView) EdgeDTO {
	return EdgeDTO{
		From: e.From, To: e.To, Status: e.Status.String(), Rel: e.Rel.String(),
		Count: e.Count, LeakCount: e.LeakCount, HijackCount: e.HijackCount,
		LastEvent: rfc3339(e.LastEvent), LastEventId: string(e.LastEventID),
	}
}

func topologyViewToDTO(v topology.SnapshotView) TopologyDTO {
	nodes := make([]NodeDTO, 0, len(v.Nodes))
	for _, n := range v.Nodes {
		nodes = append(nodes, nodeViewToDTO(n))
	}
	edges := make([]EdgeDTO, 0, len(v.Edges))
	for _, e := range v.Edges {
		edges = append(edges, edgeViewToDTO(e))
	}
	return TopologyDTO{
		Nodes: nodes, Edges: edges, NodeCount: len(nodes), EdgeCount: len(edges),
		Generated: rfc3339(v.GeneratedAt),
	}
}

func statsViewToDTO(s topology.StatsView) StatsDTO {
	origins := make([]OriginStatDTO, 0, len(s.TopOrigins))
	for _, o := range s.TopOrigins {
		tp := o.Throughput
		if tp == nil {
			tp = []int{}
		}
		origins = append(origins, OriginStatDTO{
			ASN: o.ASN, Name: o.Name, PrefixCount: o.PrefixCount, RpkiStatus: o.RPKI.String(),
			Valid: o.Valid, Invalid: o.Invalid, NotFound: o.NotFound, Throughput: tp,
		})
	}
	return StatsDTO{
		TotalEvents: s.TotalEvents, Announces: s.Announces, Withdraws: s.Withdraws,
		Leaks: s.Leaks, Hijacks: s.Hijacks, RpkiValid: s.RPKIValid, RpkiInvalid: s.RPKIInvalid,
		RpkiNotFound: s.RPKINotFound, NodeCount: s.NodeCount, EdgeCount: s.EdgeCount,
		EventsPerSec: s.EventsPerSec, TopOrigins: origins,
	}
}

func asnDetailToDTO(d topology.ASNDetailView) ASNDetailDTO {
	neighbors := make([]NeighborDTO, 0, len(d.Neighbors))
	for _, n := range d.Neighbors {
		neighbors = append(neighbors, NeighborDTO{ASN: n.ASN, Direction: n.Direction, Rel: n.Rel.String(), Status: n.Status.String()})
	}
	prefixes := d.Prefixes
	if prefixes == nil {
		prefixes = []string{}
	}
	spark := d.Sparkline
	if spark == nil {
		spark = []int{}
	}
	return ASNDetailDTO{Node: nodeViewToDTO(d.Node), Neighbors: neighbors, Prefixes: prefixes, Sparkline: spark}
}

func edgeDetailToDTO(d topology.EdgeDetailView) EdgeDetailDTO {
	return EdgeDetailDTO{Edge: edgeViewToDTO(d.Edge), Events: classifiedSliceToDTO(d.Events)}
}

func snapshotToDTO(s topology.FullSnapshot) SnapshotDTO {
	return SnapshotDTO{
		Topology: topologyViewToDTO(s.Topology),
		Events:   classifiedSliceToDTO(s.Events),
		Stats:    statsViewToDTO(s.Stats),
	}
}
