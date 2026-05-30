package relationships

import (
	"strings"
	"testing"

	"github.com/rayancheca/bgpulse/backend/internal/bgp"
)

func TestLookupDirectionAndInversion(t *testing.T) {
	// Arrange: AS100 sees AS200 as its customer.
	b := NewBuilder()
	if err := b.Add(100, 200, bgp.RelCustomer); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := b.Add(100, 300, bgp.RelPeer); err != nil {
		t.Fatalf("Add: %v", err)
	}
	s := b.Build()

	// Act + Assert: lookup is directional and inverse-symmetric.
	if got := s.Lookup(100, 200); got != bgp.RelCustomer {
		t.Errorf("Lookup(100,200) = %v, want customer", got)
	}
	if got := s.Lookup(200, 100); got != bgp.RelProvider {
		t.Errorf("Lookup(200,100) = %v, want provider", got)
	}
	if got := s.Lookup(100, 300); got != bgp.RelPeer {
		t.Errorf("Lookup(100,300) = %v, want peer", got)
	}
	if got := s.Lookup(300, 100); got != bgp.RelPeer {
		t.Errorf("Lookup(300,100) = %v, want peer (self-inverse)", got)
	}
}

func TestLookupSelfAndUnknown(t *testing.T) {
	s := NewBuilder().Build()
	if got := s.Lookup(42, 42); got != bgp.RelSibling {
		t.Errorf("Lookup(42,42) = %v, want sibling", got)
	}
	if got := s.Lookup(1, 2); got != bgp.RelUnknown {
		t.Errorf("Lookup(1,2) on empty store = %v, want unknown", got)
	}
}

func TestAddRejectsSelfAndConflict(t *testing.T) {
	b := NewBuilder()
	if err := b.Add(7, 7, bgp.RelPeer); err == nil {
		t.Error("Add(7,7) should error on self-pair")
	}
	if err := b.Add(1, 2, bgp.RelCustomer); err != nil {
		t.Fatalf("Add: %v", err)
	}
	// Consistent duplicate is fine.
	if err := b.Add(1, 2, bgp.RelCustomer); err != nil {
		t.Errorf("consistent duplicate should be accepted: %v", err)
	}
	// The same consistent fact stated from the other direction (2 sees 1 as provider).
	if err := b.Add(2, 1, bgp.RelProvider); err != nil {
		t.Errorf("equivalent inverse duplicate should be accepted: %v", err)
	}
	// Conflicting relationship is rejected.
	if err := b.Add(1, 2, bgp.RelPeer); err == nil {
		t.Error("conflicting duplicate should error")
	}
}

func TestDegree(t *testing.T) {
	b := NewBuilder()
	_ = b.Add(1, 2, bgp.RelCustomer)
	_ = b.Add(1, 3, bgp.RelCustomer)
	_ = b.Add(1, 4, bgp.RelPeer)
	s := b.Build()
	if got := s.Degree(1); got != 3 {
		t.Errorf("Degree(1) = %d, want 3", got)
	}
	if got := s.Degree(2); got != 1 {
		t.Errorf("Degree(2) = %d, want 1", got)
	}
	if got := s.Degree(999); got != 0 {
		t.Errorf("Degree(unknown) = %d, want 0", got)
	}
}

func TestLoadCAIDA(t *testing.T) {
	// AS174 provides transit to AS3356; AS174 and AS3257 peer.
	input := `# CAIDA serial-2 sample
174|3356|-1|bgp
174|3257|0

3356|65001|-1
`
	s, err := LoadCAIDA(strings.NewReader(input))
	if err != nil {
		t.Fatalf("LoadCAIDA: %v", err)
	}
	if s.Size() != 3 {
		t.Errorf("Size = %d, want 3", s.Size())
	}
	// -1 means AS1 is provider of AS2, so AS2 is AS1's customer.
	if got := s.Lookup(174, 3356); got != bgp.RelCustomer {
		t.Errorf("Lookup(174,3356) = %v, want customer", got)
	}
	if got := s.Lookup(3356, 174); got != bgp.RelProvider {
		t.Errorf("Lookup(3356,174) = %v, want provider", got)
	}
	if got := s.Lookup(174, 3257); got != bgp.RelPeer {
		t.Errorf("Lookup(174,3257) = %v, want peer", got)
	}
}

func TestLoadCAIDAErrors(t *testing.T) {
	cases := map[string]string{
		"too few fields": "174|3356\n",
		"bad AS1":        "xx|3356|-1\n",
		"bad REL code":   "174|3356|9\n",
		"non-numeric REL": "174|3356|peer\n",
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadCAIDA(strings.NewReader(input)); err == nil {
				t.Errorf("expected error for %q", input)
			}
		})
	}
}

func TestParseNames(t *testing.T) {
	b := NewBuilder()
	input := `# names
174|Cogent Communications
AS13335	Cloudflare
7018 ATT Services
`
	if err := ParseNames(b, strings.NewReader(input)); err != nil {
		t.Fatalf("ParseNames: %v", err)
	}
	s := b.Build()
	cases := map[uint32]string{
		174:   "Cogent Communications",
		13335: "Cloudflare",
		7018:  "ATT Services",
		99999: "",
	}
	for asn, want := range cases {
		if got := s.Name(asn); got != want {
			t.Errorf("Name(%d) = %q, want %q", asn, got, want)
		}
	}
}

func TestInversionPropertyOverStore(t *testing.T) {
	// Build a small graph and assert Lookup(a,b) == Lookup(b,a).Invert() for all pairs.
	b := NewBuilder()
	_ = b.Add(1, 2, bgp.RelCustomer)
	_ = b.Add(2, 3, bgp.RelPeer)
	_ = b.Add(3, 1, bgp.RelProvider)
	s := b.Build()
	asns := []uint32{1, 2, 3, 4}
	for _, a := range asns {
		for _, c := range asns {
			if s.Lookup(a, c) != s.Lookup(c, a).Invert() {
				t.Errorf("inversion broken: Lookup(%d,%d)=%v Lookup(%d,%d).Invert()=%v",
					a, c, s.Lookup(a, c), c, a, s.Lookup(c, a).Invert())
			}
		}
	}
}
