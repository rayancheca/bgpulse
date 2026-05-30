package synth

import (
	"math/rand/v2"

	"github.com/rayancheca/bgpulse/backend/internal/bgp"
)

const (
	maxClimb     = 4
	peerAtApex   = 0.40
	descendProb  = 0.35
	maxDescend   = 2
	hijackTries  = 8
)

func isTier1(asn uint32) bool {
	return asn >= tier1Base && asn < tier1Base+tier1Count
}

// reverseU32 returns a reversed copy of s.
func reverseU32(s []uint32) []uint32 {
	out := make([]uint32, len(s))
	for i, v := range s {
		out[len(s)-1-i] = v
	}
	return out
}

// validPath builds a valley-free AS_PATH in wire order (collector neighbor first,
// origin last) for a route originated by origin. The propagation path climbs the
// customer cone to a tier-1 (uphill), optionally crosses one peering link, then
// optionally descends to a customer (downhill) — exactly the (c2p)*(p2p)?(p2c)*
// shape, so the result is valley-free by construction.
func (t *Topology) validPath(origin uint32, rng *rand.Rand) []uint32 {
	prop := []uint32{origin}
	inPath := map[uint32]bool{origin: true}
	cur := origin

	// Uphill: customer -> provider until a tier-1 or no more providers.
	for len(prop) < maxClimb {
		ps := t.providersOf[cur]
		if len(ps) == 0 {
			break
		}
		next := ps[rng.IntN(len(ps))]
		if inPath[next] {
			break
		}
		prop = append(prop, next)
		inPath[next] = true
		cur = next
		if isTier1(cur) {
			break
		}
	}

	// Optional single peer link at the apex.
	if rng.Float64() < peerAtApex {
		if peers := t.peersOf[cur]; len(peers) > 0 {
			pr := peers[rng.IntN(len(peers))]
			if !inPath[pr] {
				prop = append(prop, pr)
				inPath[pr] = true
				cur = pr
			}
		}
	}

	// Optional downhill arm: provider -> customer.
	if rng.Float64() < descendProb {
		for steps := 0; steps < maxDescend; steps++ {
			cs := t.customersOf[cur]
			if len(cs) == 0 {
				break
			}
			nx := cs[rng.IntN(len(cs))]
			if inPath[nx] {
				break
			}
			prop = append(prop, nx)
			inPath[nx] = true
			cur = nx
		}
	}

	return reverseU32(prop)
}

// makeLeak emits a reproducible route leak: AS2001, a customer of both tier-1s
// AS1001 and AS1002, receives AS1001's own (RPKI-valid) prefix as a customer and
// re-announces it up to its other provider AS1002 — a provider-to-provider valley.
// Wire path [1002, 2001, 1001]; the classifier flags AS2001 as the offender.
func (g *Generator) makeLeak() bgp.UpdateEvent {
	t := g.topo
	if len(t.Tier1) < 2 || len(t.Transit) < 1 {
		return g.makeAnnounce()
	}
	origin := t.Tier1[0]
	wire := []uint32{t.Tier1[1], t.Transit[0], origin}
	return g.announce(g.pickPrefix(origin), wire)
}

// makeHijack emits a prefix hijack: a stub announces, over an otherwise plausible
// valley-free path, a prefix it does not own. The covering VRP authorizes a
// different origin, so RPKI validation returns Invalid and the event is a hijack.
func (g *Generator) makeHijack() bgp.UpdateEvent {
	t := g.topo
	if len(t.roaPrefixes) == 0 || len(t.Stubs) == 0 {
		return g.makeAnnounce()
	}
	victim := t.roaPrefixes[g.rng.IntN(len(t.roaPrefixes))]
	owner := t.prefixOwner[victim]

	hijacker := owner
	for tries := 0; tries < hijackTries && hijacker == owner; tries++ {
		hijacker = t.Stubs[g.rng.IntN(len(t.Stubs))]
	}
	if hijacker == owner {
		return g.makeAnnounce()
	}
	return g.announce(victim, t.validPath(hijacker, g.rng))
}
