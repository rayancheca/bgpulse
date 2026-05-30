package valleyfree

import (
	"testing"

	"github.com/rayancheca/bgpulse/backend/internal/bgp"
	"github.com/rayancheca/bgpulse/backend/internal/relationships"
)

// testStore builds a small but expressive topology:
//
//	A=100, B=200 : tier-1 peers
//	C=300        : customer of A and B (multi-homed transit); peers D
//	D=400        : customer of B; peers C; peers H
//	E=500        : customer of C (stub)
//	F=600        : customer of D (stub); sibling of G
//	G=700        : sibling of F
//	H=800        : peers D
func testStore(t *testing.T) *relationships.RelStore {
	t.Helper()
	b := relationships.NewBuilder()
	add := func(a, c uint32, r bgp.RelStatus) {
		if err := b.Add(a, c, r); err != nil {
			t.Fatalf("Add(%d,%d,%v): %v", a, c, r, err)
		}
	}
	add(100, 200, bgp.RelPeer)     // A-B peer
	add(100, 300, bgp.RelCustomer) // C customer of A
	add(200, 300, bgp.RelCustomer) // C customer of B
	add(200, 400, bgp.RelCustomer) // D customer of B
	add(300, 400, bgp.RelPeer)     // C-D peer
	add(300, 500, bgp.RelCustomer) // E customer of C
	add(400, 600, bgp.RelCustomer) // F customer of D
	add(400, 800, bgp.RelPeer)     // D-H peer
	add(600, 700, bgp.RelSibling)  // F-G sibling
	return b.Build()
}

func TestClassifyPath(t *testing.T) {
	rl := testStore(t)

	cases := []struct {
		name         string
		path         []uint32 // wire order: index 0 = collector neighbor, last = origin
		hasASSet     bool
		wantLeak     bool
		wantOffender uint32 // expected leaking AS when wantLeak
	}{
		{name: "pure uphill (customer cone)", path: []uint32{100, 300, 500}, wantLeak: false},
		{name: "pure downhill (provider to stub)", path: []uint32{500, 300, 100}, wantLeak: false},
		{name: "up-peer-down (valid valley-free)", path: []uint32{600, 400, 300, 500}, wantLeak: false},
		{name: "single peer at top", path: []uint32{100, 200}, wantLeak: false},
		{name: "single AS / origin direct", path: []uint32{500}, wantLeak: false},
		{name: "sibling transparent then uphill", path: []uint32{200, 400, 600, 700}, wantLeak: false},
		{name: "prepend dedup -> valid", path: []uint32{100, 100, 100, 300, 500}, wantLeak: false},

		{name: "LEAK uphill after peer", path: []uint32{100, 300, 400, 600}, wantLeak: true, wantOffender: 300},
		{name: "LEAK provider-to-provider (uphill after descent)", path: []uint32{200, 300, 100}, wantLeak: true, wantOffender: 300},
		{name: "LEAK second peer", path: []uint32{800, 400, 300, 500}, wantLeak: true, wantOffender: 400},

		{name: "unknown link never flagged", path: []uint32{999, 300, 500}, wantLeak: false},
		{name: "all-unknown path never flagged", path: []uint32{999, 888}, wantLeak: false},
		{name: "AS_SET guard prevents leak flag", path: []uint32{100, 300, 400, 600}, hasASSet: true, wantLeak: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := ClassifyPath(tc.path, tc.hasASSet, rl)

			if v.IsLeak != tc.wantLeak {
				t.Fatalf("IsLeak = %v, want %v (reason=%q)", v.IsLeak, tc.wantLeak, v.Reason)
			}
			if tc.wantLeak {
				if v.OffenderAS != tc.wantOffender {
					t.Errorf("OffenderAS = %d, want %d", v.OffenderAS, tc.wantOffender)
				}
				if v.Reason == "" {
					t.Error("leak must carry a non-empty Reason")
				}
				assertExactlyOneOffender(t, v, tc.wantOffender)
			} else {
				for _, h := range v.Hops {
					if h.IsOffender {
						t.Errorf("valley-free path must mark no offending hop, got %+v", h)
					}
				}
				if v.OffenderAS != 0 {
					t.Errorf("valley-free path OffenderAS = %d, want 0", v.OffenderAS)
				}
			}
		})
	}
}

func assertExactlyOneOffender(t *testing.T, v Verdict, wantTo uint32) {
	t.Helper()
	count := 0
	for _, h := range v.Hops {
		if h.IsOffender {
			count++
			if h.To != wantTo {
				t.Errorf("offending hop To = %d, want %d (the leaking AS)", h.To, wantTo)
			}
		}
	}
	if count != 1 {
		t.Errorf("expected exactly one offending hop, got %d", count)
	}
}

func TestAdvisoryFlags(t *testing.T) {
	rl := testStore(t)

	// Unknown link -> HadUnknown true, KnownHops counts only resolved adjacencies.
	v := ClassifyPath([]uint32{999, 300, 500}, false, rl)
	if !v.HadUnknown {
		t.Error("HadUnknown should be true when a link has no relationship data")
	}
	if v.KnownHops != 1 { // only (300,500) resolves
		t.Errorf("KnownHops = %d, want 1", v.KnownHops)
	}

	// All-unknown path -> KnownHops 0.
	v = ClassifyPath([]uint32{999, 888}, false, rl)
	if v.KnownHops != 0 || !v.HadUnknown {
		t.Errorf("all-unknown: KnownHops=%d HadUnknown=%v, want 0/true", v.KnownHops, v.HadUnknown)
	}
}

func TestHopsAreWireOrder(t *testing.T) {
	rl := testStore(t)
	// Wire path [100,300,500]; hops should be (100->300) then (300->500).
	v := ClassifyPath([]uint32{100, 300, 500}, false, rl)
	if len(v.Hops) != 2 {
		t.Fatalf("len(Hops) = %d, want 2", len(v.Hops))
	}
	if v.Hops[0].From != 100 || v.Hops[0].To != 300 {
		t.Errorf("Hops[0] = %d->%d, want 100->300", v.Hops[0].From, v.Hops[0].To)
	}
	if v.Hops[1].From != 300 || v.Hops[1].To != 500 {
		t.Errorf("Hops[1] = %d->%d, want 300->500", v.Hops[1].From, v.Hops[1].To)
	}
	// (100->300): 300 is 100's customer.
	if v.Hops[0].Rel != bgp.RelCustomer {
		t.Errorf("Hops[0].Rel = %v, want customer", v.Hops[0].Rel)
	}
}
