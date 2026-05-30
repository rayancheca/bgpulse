package classify

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/rayancheca/bgpulse/backend/internal/bgp"
	"github.com/rayancheca/bgpulse/backend/internal/relationships"
	"github.com/rayancheca/bgpulse/backend/internal/rpki"
)

func newClassifier(t *testing.T) *Classifier {
	t.Helper()
	rb := relationships.NewBuilder()
	for _, e := range []struct {
		a, c uint32
		r    bgp.RelStatus
	}{
		{100, 200, bgp.RelPeer},
		{100, 300, bgp.RelCustomer},
		{200, 300, bgp.RelCustomer},
		{200, 400, bgp.RelCustomer},
		{300, 400, bgp.RelPeer},
		{300, 500, bgp.RelCustomer},
		{400, 600, bgp.RelCustomer},
	} {
		if err := rb.Add(e.a, e.c, e.r); err != nil {
			t.Fatalf("rel Add: %v", err)
		}
	}
	vb := rpki.NewBuilder()
	if err := vb.Add(rpki.VRP{Prefix: netip.MustParsePrefix("10.0.0.0/16"), MaxLength: 24, OriginAS: 500}); err != nil {
		t.Fatalf("vrp Add: %v", err)
	}
	return New(rb.Build(), vb.Build())
}

func announce(prefix string, path []uint32, hasASSet bool) bgp.UpdateEvent {
	origin := uint32(0)
	if n := len(path); n > 0 && !hasASSet {
		origin = path[n-1]
	}
	return bgp.UpdateEvent{
		Kind:     bgp.KindAnnounce,
		Prefix:   netip.MustParsePrefix(prefix),
		ASPath:   path,
		HasASSet: hasASSet,
		OriginAS: origin,
	}
}

func TestClassifyEvent(t *testing.T) {
	c := newClassifier(t)

	cases := []struct {
		name         string
		ev           bgp.UpdateEvent
		wantVF       bgp.VFStatus
		wantRPKI     bgp.RPKIStatus
		wantOffender uint32
	}{
		{
			name:     "normal valid path + valid RPKI",
			ev:       announce("10.0.0.0/24", []uint32{100, 300, 500}, false),
			wantVF:   bgp.VFValid,
			wantRPKI: bgp.RPKIValid,
		},
		{
			name:         "leak with RPKI notfound",
			ev:           announce("203.0.113.0/24", []uint32{100, 300, 400, 600}, false),
			wantVF:       bgp.VFLeak,
			wantRPKI:     bgp.RPKINotFound,
			wantOffender: 300,
		},
		{
			name:         "hijack: wrong origin, RPKI invalid",
			ev:           announce("10.0.0.0/24", []uint32{100, 300, 600}, false),
			wantVF:       bgp.VFHijack,
			wantRPKI:     bgp.RPKIInvalid,
			wantOffender: 600,
		},
		{
			name:         "hijack precedence over leak",
			ev:           announce("10.0.0.0/24", []uint32{100, 300, 400, 600}, false),
			wantVF:       bgp.VFHijack,
			wantRPKI:     bgp.RPKIInvalid,
			wantOffender: 600, // origin is the offender for a hijack
		},
		{
			name:     "unknown topology",
			ev:       announce("198.51.100.0/24", []uint32{999, 888}, false),
			wantVF:   bgp.VFUnknown,
			wantRPKI: bgp.RPKINotFound,
		},
		{
			name:     "AS_SET origin: no RPKI hijack, not leaked",
			ev:       announce("10.0.0.0/24", []uint32{100, 300}, true),
			wantVF:   bgp.VFValid,
			wantRPKI: bgp.RPKINotFound, // origin unknown -> RPKI not evaluated
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := c.Classify(tc.ev)
			if got.VFStatus != tc.wantVF {
				t.Errorf("VFStatus = %v, want %v (reason=%q)", got.VFStatus, tc.wantVF, got.Reason)
			}
			if got.RPKIStatus != tc.wantRPKI {
				t.Errorf("RPKIStatus = %v, want %v", got.RPKIStatus, tc.wantRPKI)
			}
			if got.OffenderAS != tc.wantOffender {
				t.Errorf("OffenderAS = %d, want %d", got.OffenderAS, tc.wantOffender)
			}
		})
	}
}

func TestHijackAlsoLeaksReason(t *testing.T) {
	c := newClassifier(t)
	got := c.Classify(announce("10.0.0.0/24", []uint32{100, 300, 400, 600}, false))
	if got.VFStatus != bgp.VFHijack {
		t.Fatalf("VFStatus = %v, want hijack", got.VFStatus)
	}
	if !strings.Contains(got.Reason, "also leaks") {
		t.Errorf("reason should note the path also leaks, got %q", got.Reason)
	}
}

func TestClassifyWithdrawIsNeutral(t *testing.T) {
	c := newClassifier(t)
	ev := bgp.UpdateEvent{Kind: bgp.KindWithdraw, Prefix: netip.MustParsePrefix("10.0.0.0/24")}
	got := c.Classify(ev)
	if got.VFStatus != bgp.VFValid || got.RPKIStatus != bgp.RPKINotFound {
		t.Errorf("withdraw classified %v/%v, want valid/notfound", got.VFStatus, got.RPKIStatus)
	}
	if len(got.Hops) != 0 || got.OffenderAS != 0 {
		t.Errorf("withdraw should have no hops/offender, got %d hops offender=%d", len(got.Hops), got.OffenderAS)
	}
}
