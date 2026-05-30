// Package relationships holds the AS-relationship graph used by the Gao-Rexford
// valley-free classifier. Each unordered AS pair has one relationship; the store
// keeps it canonically (from the numerically smaller AS's perspective) and derives
// the inverse on lookup, so Lookup(a, b) == Lookup(b, a).Invert() by construction.
//
// The store is immutable after Build. Relationships come either from a real CAIDA
// serial-2 file (see ParseCAIDA) or from the deterministic synthetic topology.
package relationships

import (
	"fmt"

	"github.com/rayancheca/bgpulse/backend/internal/bgp"
)

// pairKey is a canonical unordered AS pair with lo <= hi.
type pairKey struct {
	lo uint32
	hi uint32
}

// RelStore is an immutable AS-relationship graph. Concurrent reads are safe.
type RelStore struct {
	// edges maps a canonical (lo, hi) pair to the relationship FROM lo toward hi.
	edges map[pairKey]bgp.RelStatus
	// names maps an ASN to a human-readable name ("" if unknown).
	names map[uint32]string
	// degree maps an ASN to its distinct-neighbor count.
	degree map[uint32]int
}

// Lookup returns the relationship of a toward b. It returns RelSibling for a == b
// (an AS is trivially same-org with itself) and RelUnknown when the pair is absent.
func (s *RelStore) Lookup(a, b uint32) bgp.RelStatus {
	if a == b {
		return bgp.RelSibling
	}
	lo, hi := a, b
	swapped := false
	if lo > hi {
		lo, hi = hi, lo
		swapped = true
	}
	rt, ok := s.edges[pairKey{lo, hi}]
	if !ok {
		return bgp.RelUnknown
	}
	if swapped {
		return rt.Invert()
	}
	return rt
}

// Name returns the human-readable name for an ASN, or "" if unknown.
func (s *RelStore) Name(asn uint32) string { return s.names[asn] }

// Degree returns the number of distinct neighbors of an ASN (0 if unknown).
func (s *RelStore) Degree(asn uint32) int { return s.degree[asn] }

// Size returns the number of distinct relationship edges.
func (s *RelStore) Size() int { return len(s.edges) }

// Builder accumulates relationships and names, then seals them into an immutable
// RelStore. It is not safe for concurrent use.
type Builder struct {
	edges     map[pairKey]bgp.RelStatus
	neighbors map[uint32]map[uint32]struct{}
	names     map[uint32]string
}

// NewBuilder returns an empty Builder.
func NewBuilder() *Builder {
	return &Builder{
		edges:     make(map[pairKey]bgp.RelStatus),
		neighbors: make(map[uint32]map[uint32]struct{}),
		names:     make(map[uint32]string),
	}
}

// Add records that, from a's perspective, b is rel (e.g. RelCustomer means b is
// a's customer). The relationship is normalized to canonical (lo, hi) form. Adding
// the same pair twice with an inconsistent relationship is an error; consistent
// duplicates are ignored. Self-pairs (a == b) are rejected.
func (b *Builder) Add(a, c uint32, rel bgp.RelStatus) error {
	if a == c {
		return fmt.Errorf("relationships: self-pair AS%d", a)
	}
	lo, hi, canon := a, c, rel
	if lo > hi {
		lo, hi = hi, lo
		canon = rel.Invert() // store from lo's perspective
	}
	key := pairKey{lo, hi}
	if existing, ok := b.edges[key]; ok {
		if existing != canon {
			return fmt.Errorf("relationships: conflicting relationship for AS%d|AS%d (%s vs %s)",
				lo, hi, existing, canon)
		}
		return nil
	}
	b.edges[key] = canon
	b.addNeighbor(a, c)
	b.addNeighbor(c, a)
	return nil
}

func (b *Builder) addNeighbor(a, c uint32) {
	set, ok := b.neighbors[a]
	if !ok {
		set = make(map[uint32]struct{})
		b.neighbors[a] = set
	}
	set[c] = struct{}{}
}

// SetName attaches a human-readable name to an ASN (last write wins).
func (b *Builder) SetName(asn uint32, name string) {
	if name != "" {
		b.names[asn] = name
	}
}

// Build seals the builder into an immutable RelStore.
func (b *Builder) Build() *RelStore {
	degree := make(map[uint32]int, len(b.neighbors))
	for asn, set := range b.neighbors {
		degree[asn] = len(set)
	}
	return &RelStore{edges: b.edges, names: b.names, degree: degree}
}
