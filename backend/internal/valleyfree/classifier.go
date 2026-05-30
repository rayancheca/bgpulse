// Package valleyfree implements the Gao-Rexford valley-free classifier: given an
// AS_PATH and an AS-relationship oracle, it decides whether the path obeys the
// valley-free export constraint and, if not, pinpoints the offending AS hop.
//
// The rule. Reading the path in the direction of route propagation (origin first,
// collector last), the sequence of inter-AS relationship "moves" must match
//
//	(c2p)*  (p2p | p2c)?  (p2c)*
//
// i.e. an uphill run of customer->provider links, then at most one peer link or the
// first downhill (provider->customer) link, then a downhill run. Once a route has
// peered or started descending it may never climb again and may never take a second
// peer link. Any move that breaks this monotone "up then down" shape is a valley —
// a route leak. Sibling links are phase-transparent; unknown and AS_SET-derived
// links are never used to assert a violation (no false leaks).
package valleyfree

import "github.com/rayancheca/bgpulse/backend/internal/bgp"

// RelLookup is the relationship oracle the classifier consults. Lookup(a, b)
// returns a's relationship toward b. Both *relationships.RelStore and the synthetic
// topology's derived store satisfy this.
type RelLookup interface {
	Lookup(a, b uint32) bgp.RelStatus
}

// Verdict is the result of classifying one AS_PATH. The caller (the classify layer)
// combines IsLeak with RPKI to produce the final bgp.VFStatus.
type Verdict struct {
	IsLeak     bool          // true if the path contains a valley
	OffenderAS uint32        // the leaking AS (apex of the valley); 0 if valley-free
	Reason     string        // short human-readable explanation; "" if valley-free
	Hops       []bgp.PathHop // per-adjacency annotation in wire order (collector->origin)
	HadUnknown bool          // at least one adjacency had no relationship data
	KnownHops  int           // number of adjacencies with a known relationship
}

// walkPhase tracks where we are in the up-then-down shape.
type walkPhase uint8

const (
	phaseUp   walkPhase = iota // still climbing customer->provider links
	phaseDown                  // have peered or begun descending; only p2c allowed
)

// ClassifyPath classifies a wire-order AS_PATH (index 0 nearest the collector, last
// element the origin). hasASSet marks that the original path contained an AS_SET
// aggregation segment; such paths are never flagged as leaks because an unordered
// set has no provable relationship sequence. The relationship oracle rl is read-only.
func ClassifyPath(path []uint32, hasASSet bool, rl RelLookup) Verdict {
	seq := dedupeConsecutive(path)
	hops, knownHops, hadUnknown := buildHops(seq, rl)
	verdict := Verdict{Hops: hops, HadUnknown: hadUnknown, KnownHops: knownHops}

	// A single AS (or empty path) has no inter-AS move; an aggregated path is
	// unprovable. Either way it is not a leak.
	if len(seq) <= 1 || hasASSet {
		return verdict
	}

	offHop, offender, reason := findValley(seq, rl)
	if offHop >= 0 {
		verdict.IsLeak = true
		verdict.OffenderAS = offender
		verdict.Reason = reason
		verdict.Hops[offHop].IsOffender = true
	}
	return verdict
}

// dedupeConsecutive collapses runs of the same ASN (AS_PATH prepending) so the
// relationship walk and hop reporting use one entry per distinct adjacency.
func dedupeConsecutive(path []uint32) []uint32 {
	if len(path) == 0 {
		return nil
	}
	out := make([]uint32, 0, len(path))
	for _, asn := range path {
		if n := len(out); n > 0 && out[n-1] == asn {
			continue
		}
		out = append(out, asn)
	}
	return out
}

// buildHops produces the wire-order adjacency annotations. hop.From is the AS toward
// the collector, hop.To the AS toward the origin; Rel is From's relationship to To.
func buildHops(seq []uint32, rl RelLookup) (hops []bgp.PathHop, knownHops int, hadUnknown bool) {
	if len(seq) < 2 {
		return nil, 0, false
	}
	hops = make([]bgp.PathHop, 0, len(seq)-1)
	for k := 0; k+1 < len(seq); k++ {
		from, to := seq[k], seq[k+1]
		rel := rl.Lookup(from, to)
		if rel == bgp.RelUnknown {
			hadUnknown = true
		} else {
			knownHops++
		}
		hops = append(hops, bgp.PathHop{From: from, To: to, Rel: rel})
	}
	return hops, knownHops, hadUnknown
}

// findValley walks the path in propagation order (origin -> collector, i.e.
// decreasing wire index) and returns the wire index of the first offending
// adjacency (or -1), the leaking AS, and a reason. It stops at the first violation.
func findValley(seq []uint32, rl RelLookup) (offHop int, offender uint32, reason string) {
	phase := phaseUp
	for i := len(seq) - 1; i > 0; i-- {
		u, v := seq[i], seq[i-1] // u nearer origin, v nearer collector
		switch rl.Lookup(u, v) {
		case bgp.RelSibling, bgp.RelUnknown:
			// Phase-transparent: siblings don't move up or down, and we refuse to
			// assert a valley across a link we have no data for.
			continue
		case bgp.RelProvider: // v is u's provider => uphill (customer->provider)
			if phase == phaseDown {
				return i - 1, u, "route leaked to a provider after peering or descending"
			}
		case bgp.RelPeer:
			if phase == phaseDown {
				return i - 1, u, "route leaked to a second peer (or a peer after descending)"
			}
			phase = phaseDown
		case bgp.RelCustomer: // v is u's customer => downhill (provider->customer)
			phase = phaseDown
		}
	}
	return -1, 0, ""
}
