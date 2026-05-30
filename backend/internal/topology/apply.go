package topology

import (
	"time"

	"github.com/rayancheca/bgpulse/backend/internal/bgp"
)

// maxSamplesPerEdge bounds the per-edge FIFO of recent event ids kept for drill-down.
const maxSamplesPerEdge = 15

// apply is the single mutation path for the graph; it runs only on the Aggregator
// goroutine, so it needs no locking.
func (a *Aggregator) apply(ev bgp.ClassifiedEvent) {
	e := ev.Event
	now := e.Timestamp

	if e.Kind == bgp.KindAnnounce && e.OriginAS != 0 {
		a.applyOrigin(ev, now)
	}
	if e.Kind == bgp.KindAnnounce {
		a.applyEdges(ev, now)
	}
	a.applyRIB(ev)

	a.ring.push(ev)
	a.stats.record(ev)
}

// applyOrigin updates the origin node's RPKI tallies, throughput series, and activity.
func (a *Aggregator) applyOrigin(ev bgp.ClassifiedEvent, now time.Time) {
	n := a.graph.node(ev.Event.OriginAS, now, a.name(ev.Event.OriginAS))
	n.LastSeen = now
	n.Events++
	n.series.add(now)
	switch ev.RPKIStatus {
	case bgp.RPKIValid:
		n.RPKI.Valid++
	case bgp.RPKIInvalid:
		n.RPKI.Invalid++
	case bgp.RPKINotFound:
		n.RPKI.NotFound++
	}
	n.worstRPKI = worseRPKI(n.worstRPKI, ev.RPKIStatus)
}

// applyEdges ensures the path's nodes exist and upserts each directed adjacency.
func (a *Aggregator) applyEdges(ev bgp.ClassifiedEvent, now time.Time) {
	path := ev.Event.ASPath
	for i := 0; i+1 < len(path); i++ {
		from, to := path[i], path[i+1]
		if from == to {
			continue // AS_PATH prepending: not a real adjacency
		}
		a.graph.node(from, now, a.name(from)).LastSeen = now
		a.graph.node(to, now, a.name(to)).LastSeen = now
		a.upsertEdge(from, to, ev, now)
	}
}

func (a *Aggregator) upsertEdge(from, to uint32, ev bgp.ClassifiedEvent, now time.Time) {
	key := EdgeKey{From: from, To: to}
	ed, ok := a.graph.Edges[key]
	if !ok {
		ed = &Edge{Key: key, Rel: relForHop(ev.Hops, from, to)}
		a.graph.Edges[key] = ed
	}
	ed.Count++
	switch ev.VFStatus {
	case bgp.VFLeak:
		ed.LeakCount++
	case bgp.VFHijack:
		ed.HijackCount++
	}
	// The edge shows its most recent classification, so anomalies flash and then
	// return to normal as benign traffic resumes on the adjacency.
	ed.Status = ev.VFStatus
	ed.LastEvent = now
	ed.LastEventID = ev.Event.ID
	ed.samples = appendSample(ed.samples, ev.Event.ID)
}

// applyRIB keeps prefix ownership correct: announces (re)assign a prefix to its
// origin, handling origin changes; withdraws release it. Prefix counts are derived
// from a set, so they never go negative.
func (a *Aggregator) applyRIB(ev bgp.ClassifiedEvent) {
	cidr := ev.Event.Prefix.String()
	switch ev.Event.Kind {
	case bgp.KindAnnounce:
		if ev.Event.OriginAS == 0 {
			return // unverifiable origin: not tracked in the RIB
		}
		if prev, ok := a.graph.rib[cidr]; ok && prev.originAS != ev.Event.OriginAS {
			if old := a.graph.Nodes[prev.originAS]; old != nil {
				delete(old.prefixes, cidr)
			}
		}
		a.graph.rib[cidr] = &ribEntry{originAS: ev.Event.OriginAS}
		if n := a.graph.Nodes[ev.Event.OriginAS]; n != nil {
			n.prefixes[cidr] = struct{}{}
		}
	case bgp.KindWithdraw:
		if prev, ok := a.graph.rib[cidr]; ok {
			if n := a.graph.Nodes[prev.originAS]; n != nil {
				delete(n.prefixes, cidr)
			}
			delete(a.graph.rib, cidr)
		}
	}
}

// relForHop returns the inferred relationship for the from->to hop if present.
func relForHop(hops []bgp.PathHop, from, to uint32) bgp.RelStatus {
	for _, h := range hops {
		if h.From == from && h.To == to {
			return h.Rel
		}
	}
	return bgp.RelUnknown
}

// worseRPKI picks the more alarming representative status: Invalid beats Valid beats
// NotFound, so a node with any Invalid announcement reads as compromised.
func worseRPKI(a, b bgp.RPKIStatus) bgp.RPKIStatus {
	if a == bgp.RPKIInvalid || b == bgp.RPKIInvalid {
		return bgp.RPKIInvalid
	}
	if a == bgp.RPKIValid || b == bgp.RPKIValid {
		return bgp.RPKIValid
	}
	return bgp.RPKINotFound
}

func appendSample(s []bgp.EventID, id bgp.EventID) []bgp.EventID {
	s = append(s, id)
	if len(s) > maxSamplesPerEdge {
		s = s[len(s)-maxSamplesPerEdge:]
	}
	return s
}
