// Package synth is the deterministic synthetic BGP stream used in demo mode. It
// builds ONE canonical tiered AS topology and derives, from that single source of
// truth, both the AS-relationship store the valley-free classifier consults and the
// RPKI VRP set the origin validator consults. The generator then emits a realistic
// valley-free baseline and injects real route leaks and prefix hijacks on a
// schedule, so the classifiers reliably light up. Everything is driven by a seeded
// math/rand/v2 PCG, so a given seed produces a byte-identical event stream.
package synth

import (
	"fmt"
	"math/rand/v2"
	"net/netip"

	"github.com/rayancheca/bgpulse/backend/internal/relationships"
	"github.com/rayancheca/bgpulse/backend/internal/rpki"
	"github.com/rayancheca/bgpulse/backend/internal/bgp"
)

// DefaultSeed is the demo PRNG seed. The cute value is the ASCII for "bgpulse".
const DefaultSeed uint64 = 0x6267_7075_6c73_65

const (
	tier1Base    = 1001
	tier1Count   = 6
	transitBase  = 2001
	transitCount = 24
	stubBase     = 3001
	stubCount    = 120

	peerProb          = 0.15 // probability two same-tier transit ASes peer
	extraUpstreamProb = 0.30 // probability a stub buys from a second provider
	siblingEvery      = 17   // every Nth stub is a sibling of its successor
	secondPrefixProb  = 0.40 // probability a stub originates a second prefix
	roaSkipEvery      = 5    // every Nth originated prefix gets no ROA (-> NotFound)

	goldenGamma = 0x9E3779B97F4A7C15 // PCG stream offset
)

var tier1Names = [tier1Count]string{"Arelion", "Lumen", "Cogent", "GTT", "Telia", "NTT"}

// Topology is the immutable canonical demo graph plus the stores derived from it.
type Topology struct {
	Tier1   []uint32
	Transit []uint32
	Stubs   []uint32

	rel *relationships.RelStore
	vrp *rpki.VRPStore

	providersOf map[uint32][]uint32
	customersOf map[uint32][]uint32
	peersOf     map[uint32][]uint32

	ownerPrefixes map[uint32][]netip.Prefix
	prefixOwner   map[netip.Prefix]uint32
	originables   []uint32 // ASes that originate at least one prefix
	roaPrefixes   []netip.Prefix
}

// Rel returns the relationship store derived from this topology.
func (t *Topology) Rel() *relationships.RelStore { return t.rel }

// VRP returns the RPKI VRP store derived from this topology.
func (t *Topology) VRP() *rpki.VRPStore { return t.vrp }

// builder accumulates the topology during deterministic construction.
type builder struct {
	rng *rand.Rand
	rel *relationships.Builder
	t   *Topology
}

// BuildDefault builds the default demo topology for the given seed.
func BuildDefault(seed uint64) *Topology {
	b := &builder{
		rng: rand.New(rand.NewPCG(seed, seed^goldenGamma)),
		rel: relationships.NewBuilder(),
		t: &Topology{
			providersOf:   map[uint32][]uint32{},
			customersOf:   map[uint32][]uint32{},
			peersOf:       map[uint32][]uint32{},
			ownerPrefixes: map[uint32][]netip.Prefix{},
			prefixOwner:   map[netip.Prefix]uint32{},
		},
	}
	b.assignASNs()
	b.linkTier1()
	b.linkTransit()
	b.linkStubs()
	b.allocatePrefixes()
	b.t.rel = b.finalizeRel()
	b.t.vrp = b.buildVRP()
	return b.t
}

func (b *builder) assignASNs() {
	for i := 0; i < tier1Count; i++ {
		b.t.Tier1 = append(b.t.Tier1, uint32(tier1Base+i))
	}
	for i := 0; i < transitCount; i++ {
		b.t.Transit = append(b.t.Transit, uint32(transitBase+i))
	}
	for i := 0; i < stubCount; i++ {
		b.t.Stubs = append(b.t.Stubs, uint32(stubBase+i))
	}
}

// edge records a relationship both in the relationships builder and in the
// adjacency maps used for path construction. rel is a's view of b.
func (b *builder) edge(a, c uint32, rel bgp.RelStatus) {
	if err := b.rel.Add(a, c, rel); err != nil {
		return // conflicting duplicate: keep the first (deterministic)
	}
	switch rel {
	case bgp.RelCustomer: // c is a's customer; a is c's provider
		b.t.customersOf[a] = append(b.t.customersOf[a], c)
		b.t.providersOf[c] = append(b.t.providersOf[c], a)
	case bgp.RelProvider: // c is a's provider; a is c's customer
		b.t.providersOf[a] = append(b.t.providersOf[a], c)
		b.t.customersOf[c] = append(b.t.customersOf[c], a)
	case bgp.RelPeer:
		b.t.peersOf[a] = append(b.t.peersOf[a], c)
		b.t.peersOf[c] = append(b.t.peersOf[c], a)
	}
}

func (b *builder) linkTier1() {
	for i := 0; i < len(b.t.Tier1); i++ {
		for j := i + 1; j < len(b.t.Tier1); j++ {
			b.edge(b.t.Tier1[i], b.t.Tier1[j], bgp.RelPeer)
		}
	}
}

func (b *builder) linkTransit() {
	for idx, t := range b.t.Transit {
		// Transit index 0 (AS2001) is forced multi-homed to the first two tier-1s so
		// the leak scenario (provider-to-provider re-export at AS2001) is reproducible.
		if idx == 0 {
			b.edge(t, b.t.Tier1[0], bgp.RelProvider)
			b.edge(t, b.t.Tier1[1], bgp.RelProvider)
		} else {
			n := 1 + b.rng.IntN(2) // 1 or 2 upstreams
			picked := b.pickDistinct(b.t.Tier1, n)
			for _, p := range picked {
				b.edge(t, p, bgp.RelProvider)
			}
		}
	}
	for i := 0; i < len(b.t.Transit); i++ {
		for j := i + 1; j < len(b.t.Transit); j++ {
			if b.rng.Float64() < peerProb {
				b.edge(b.t.Transit[i], b.t.Transit[j], bgp.RelPeer)
			}
		}
	}
}

func (b *builder) linkStubs() {
	for idx, s := range b.t.Stubs {
		first := b.t.Transit[b.rng.IntN(len(b.t.Transit))]
		b.edge(s, first, bgp.RelProvider)
		if b.rng.Float64() < extraUpstreamProb {
			second := b.t.Transit[b.rng.IntN(len(b.t.Transit))]
			if second != first {
				b.edge(s, second, bgp.RelProvider)
			}
		}
		if idx%siblingEvery == 0 && idx+1 < len(b.t.Stubs) {
			b.edge(s, b.t.Stubs[idx+1], bgp.RelSibling)
		}
	}
}

// pickDistinct returns n distinct elements drawn from pool using the seeded RNG.
func (b *builder) pickDistinct(pool []uint32, n int) []uint32 {
	if n >= len(pool) {
		out := make([]uint32, len(pool))
		copy(out, pool)
		return out
	}
	seen := map[uint32]struct{}{}
	out := make([]uint32, 0, n)
	for len(out) < n {
		v := pool[b.rng.IntN(len(pool))]
		if _, dup := seen[v]; dup {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func (b *builder) finalizeRel() *relationships.RelStore {
	for i, asn := range b.t.Tier1 {
		b.rel.SetName(asn, tier1Names[i])
	}
	for _, asn := range b.t.Transit {
		b.rel.SetName(asn, fmt.Sprintf("Transit-%d", asn))
	}
	for _, asn := range b.t.Stubs {
		b.rel.SetName(asn, fmt.Sprintf("AS%d", asn))
	}
	return b.rel.Build()
}
