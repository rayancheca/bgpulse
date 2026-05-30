package rpki

import (
	"fmt"
	"net/netip"

	"github.com/rayancheca/bgpulse/backend/internal/bgp"
)

const (
	maxBitsV4 = 32
	maxBitsV6 = 128
)

// trieNode is a binary radix-trie node. A node reached by following d address bits
// from the root holds the VRPs whose prefix length is exactly d.
type trieNode struct {
	children [2]*trieNode
	vrps     []VRP
}

// VRPStore is an immutable longest/all-prefix index of VRPs, with separate tries
// for IPv4 and IPv6. It is safe for concurrent reads; live updates replace the
// whole store via an atomic pointer rather than mutating in place.
type VRPStore struct {
	v4   *trieNode
	v6   *trieNode
	size int
}

// Size returns the number of VRPs in the store.
func (s *VRPStore) Size() int { return s.size }

// bitAt returns the d-th most-significant bit (0-indexed) of an address.
func bitAt(a netip.Addr, d int) int {
	if a.Is4() {
		b := a.As4()
		return int((b[d/8] >> (7 - uint(d%8))) & 1)
	}
	b := a.As16()
	return int((b[d/8] >> (7 - uint(d%8))) & 1)
}

// Validate performs RFC 6811 route origin validation for an announced (prefix,
// origin). It descends the trie along the announced prefix's bits; every VRP found
// on that descent covers the announcement. A covering VRP matches when the
// announcement length is within the VRP's maxLength and the origins are equal (and
// non-zero). Valid if any match; Invalid if covered but unmatched; NotFound if no
// covering VRP.
func (s *VRPStore) Validate(prefix netip.Prefix, origin uint32) Result {
	if !prefix.IsValid() {
		return Result{Status: bgp.RPKINotFound}
	}
	addr := prefix.Addr().Unmap()
	root := s.v4
	if addr.Is6() {
		root = s.v6
	}
	pBits := prefix.Bits()

	covering := 0
	var matched *VRP
	for node, depth := root, 0; node != nil; depth++ {
		for i := range node.vrps {
			v := &node.vrps[i]
			covering++
			if pBits <= int(v.MaxLength) && origin != 0 && v.OriginAS == origin {
				matched = v
			}
		}
		if depth == pBits {
			break
		}
		node = node.children[bitAt(addr, depth)]
	}

	switch {
	case matched != nil:
		return Result{Status: bgp.RPKIValid, MatchedVRP: matched, CoveringVRPs: covering}
	case covering > 0:
		return Result{Status: bgp.RPKIInvalid, CoveringVRPs: covering}
	default:
		return Result{Status: bgp.RPKINotFound}
	}
}

// Builder accumulates VRPs and seals them into an immutable VRPStore. It is not
// safe for concurrent use.
type Builder struct {
	v4   *trieNode
	v6   *trieNode
	size int
	seen map[string]struct{}
}

// NewBuilder returns an empty Builder.
func NewBuilder() *Builder {
	return &Builder{seen: make(map[string]struct{})}
}

// Add inserts a VRP. The prefix is masked; MaxLength must satisfy
// Prefix.Bits() <= MaxLength <= family max. Exact duplicates are ignored.
func (b *Builder) Add(v VRP) error {
	if !v.Prefix.IsValid() {
		return fmt.Errorf("rpki: invalid prefix %v", v.Prefix)
	}
	v.Prefix = v.Prefix.Masked()
	addr := v.Prefix.Addr().Unmap()
	bits := maxBitsV4
	rootp := &b.v4
	if addr.Is6() {
		bits = maxBitsV6
		rootp = &b.v6
	}
	pl := v.Prefix.Bits()
	if int(v.MaxLength) < pl || int(v.MaxLength) > bits {
		return fmt.Errorf("rpki: maxLength %d out of range [%d,%d] for %v", v.MaxLength, pl, bits, v.Prefix)
	}

	key := fmt.Sprintf("%s|%d|%d", v.Prefix, v.MaxLength, v.OriginAS)
	if _, dup := b.seen[key]; dup {
		return nil
	}
	b.seen[key] = struct{}{}

	if *rootp == nil {
		*rootp = &trieNode{}
	}
	node := *rootp
	for d := 0; d < pl; d++ {
		bit := bitAt(addr, d)
		if node.children[bit] == nil {
			node.children[bit] = &trieNode{}
		}
		node = node.children[bit]
	}
	node.vrps = append(node.vrps, v)
	b.size++
	return nil
}

// Build seals the builder into an immutable VRPStore.
func (b *Builder) Build() *VRPStore {
	return &VRPStore{v4: b.v4, v6: b.v6, size: b.size}
}
