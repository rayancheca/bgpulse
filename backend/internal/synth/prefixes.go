package synth

import (
	"net/netip"

	"github.com/rayancheca/bgpulse/backend/internal/rpki"
)

// p16 builds a masked /16 prefix a.b.0.0/16.
func p16(a, b byte) netip.Prefix {
	return netip.PrefixFrom(netip.AddrFrom4([4]byte{a, b, 0, 0}), 16).Masked()
}

// p24 builds a masked /24 prefix a.b.c.0/24.
func p24(a, b, c byte) netip.Prefix {
	return netip.PrefixFrom(netip.AddrFrom4([4]byte{a, b, c, 0}), 24).Masked()
}

// allocatePrefixes deterministically assigns prefixes to ASes: each tier-1 a /16
// in 11.0.0.0/8, each transit a /16 in 12.0.0.0/8, each stub a /24 in 100.0.0.0/16
// (plus a chance of a second in 101.0.0.0/16). Every roaSkipEvery-th prefix is left
// without a ROA so the demo shows a realistic mix of Valid and NotFound.
func (b *builder) allocatePrefixes() {
	idx := 0
	alloc := func(owner uint32, p netip.Prefix) {
		if _, exists := b.t.prefixOwner[p]; exists {
			return
		}
		b.t.prefixOwner[p] = owner
		b.t.ownerPrefixes[owner] = append(b.t.ownerPrefixes[owner], p)
		if idx%roaSkipEvery != roaSkipEvery-1 {
			b.t.roaPrefixes = append(b.t.roaPrefixes, p)
		}
		idx++
	}

	for i, asn := range b.t.Tier1 {
		alloc(asn, p16(11, byte(i)))
	}
	for m, asn := range b.t.Transit {
		alloc(asn, p16(12, byte(m)))
	}
	for k, asn := range b.t.Stubs {
		alloc(asn, p24(100, 0, byte(k)))
		if b.rng.Float64() < secondPrefixProb {
			alloc(asn, p24(101, 0, byte(k)))
		}
	}

	for _, asn := range b.allASNsInOrder() {
		if len(b.t.ownerPrefixes[asn]) > 0 {
			b.t.originables = append(b.t.originables, asn)
		}
	}
}

// allASNsInOrder returns every ASN tier-1, then transit, then stub, ascending.
func (b *builder) allASNsInOrder() []uint32 {
	all := make([]uint32, 0, len(b.t.Tier1)+len(b.t.Transit)+len(b.t.Stubs))
	all = append(all, b.t.Tier1...)
	all = append(all, b.t.Transit...)
	all = append(all, b.t.Stubs...)
	return all
}

// buildVRP creates the VRP store: one exact-length VRP per ROA'd prefix authorizing
// its true owner. Any wrong-origin or more-specific announcement of these prefixes
// is therefore RPKI-Invalid, which is exactly what the hijack scenario relies on.
func (b *builder) buildVRP() *rpki.VRPStore {
	vb := rpki.NewBuilder()
	for _, p := range b.t.roaPrefixes {
		owner := b.t.prefixOwner[p]
		_ = vb.Add(rpki.VRP{Prefix: p, MaxLength: uint8(p.Bits()), OriginAS: owner})
	}
	return vb.Build()
}
