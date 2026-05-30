package rpki

import (
	"net/netip"
	"testing"

	"github.com/rayancheca/bgpulse/backend/internal/bgp"
)

// build constructs a VRPStore from (cidr, maxLen, origin) triples.
func build(t *testing.T, vrps ...VRP) *VRPStore {
	t.Helper()
	b := NewBuilder()
	for _, v := range vrps {
		if err := b.Add(v); err != nil {
			t.Fatalf("Add(%v): %v", v, err)
		}
	}
	return b.Build()
}

func vrp(cidr string, maxLen uint8, origin uint32) VRP {
	return VRP{Prefix: netip.MustParsePrefix(cidr), MaxLength: maxLen, OriginAS: origin}
}

func TestValidate(t *testing.T) {
	store := build(t,
		vrp("10.0.0.0/16", 24, 500),    // 500 may originate 10.0.0.0/16 up to /24
		vrp("10.0.0.0/8", 24, 999),     // a less-specific ROA owned by a different AS
		vrp("192.0.2.0/24", 24, 0),     // AS0 disavowal
		vrp("2001:db8::/32", 48, 500),  // IPv6
	)

	cases := []struct {
		name   string
		cidr   string
		origin uint32
		want   bgp.RPKIStatus
	}{
		{"valid within maxLength", "10.0.0.0/24", 500, bgp.RPKIValid},
		{"valid exact length", "10.0.0.0/16", 500, bgp.RPKIValid},
		{"invalid wrong origin", "10.0.0.0/24", 600, bgp.RPKIInvalid},
		// THE RFC 6811 subtlety: /25 is contained by the /16 ROA (covered) but more
		// specific than its maxLength 24 -> not a match -> Invalid, NOT NotFound.
		{"invalid maxLength exceeded", "10.0.0.0/25", 500, bgp.RPKIInvalid},
		{"invalid maxLength exceeded + wrong origin", "10.0.0.0/25", 600, bgp.RPKIInvalid},
		{"notfound uncovered prefix", "172.16.0.0/16", 500, bgp.RPKINotFound},
		{"notfound disjoint within different /8", "11.0.0.0/16", 500, bgp.RPKINotFound},
		{"AS0 disavowal -> invalid", "192.0.2.0/24", 64500, bgp.RPKIInvalid},
		{"announce origin AS0 -> invalid when covered", "10.0.0.0/24", 0, bgp.RPKIInvalid},
		{"valid wins over other covering ROA", "10.0.5.0/24", 500, bgp.RPKIValid},
		{"covered only by other-AS ROA -> invalid", "10.128.0.0/12", 500, bgp.RPKIInvalid},
		{"ipv6 valid within maxLength", "2001:db8:1::/48", 500, bgp.RPKIValid},
		{"ipv6 invalid wrong origin", "2001:db8:1::/48", 600, bgp.RPKIInvalid},
		{"ipv6 notfound", "2001:dead::/32", 500, bgp.RPKINotFound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := store.Validate(netip.MustParsePrefix(tc.cidr), tc.origin)
			if got.Status != tc.want {
				t.Errorf("Validate(%s, %d) = %v (covering=%d), want %v",
					tc.cidr, tc.origin, got.Status, got.CoveringVRPs, tc.want)
			}
		})
	}
}

func TestValidateMoreSpecificROADoesNotCover(t *testing.T) {
	// A ROA for a /24 must NOT cover an announcement of the enclosing /16.
	store := build(t, vrp("10.0.0.0/24", 24, 500))
	got := store.Validate(netip.MustParsePrefix("10.0.0.0/16"), 500)
	if got.Status != bgp.RPKINotFound {
		t.Errorf("/16 announce vs /24 ROA = %v, want NotFound", got.Status)
	}
}

func TestValidateMatchedVRP(t *testing.T) {
	store := build(t, vrp("10.0.0.0/16", 24, 500))
	got := store.Validate(netip.MustParsePrefix("10.0.0.0/24"), 500)
	if got.MatchedVRP == nil {
		t.Fatal("Valid result must carry MatchedVRP")
	}
	if got.MatchedVRP.OriginAS != 500 {
		t.Errorf("MatchedVRP.OriginAS = %d, want 500", got.MatchedVRP.OriginAS)
	}
}

func TestBuilderAddRejectsBadMaxLength(t *testing.T) {
	b := NewBuilder()
	// maxLength below prefix length.
	if err := b.Add(VRP{Prefix: netip.MustParsePrefix("10.0.0.0/24"), MaxLength: 16, OriginAS: 1}); err == nil {
		t.Error("expected error for maxLength < prefix length")
	}
	// maxLength above family max.
	if err := b.Add(VRP{Prefix: netip.MustParsePrefix("10.0.0.0/24"), MaxLength: 40, OriginAS: 1}); err == nil {
		t.Error("expected error for maxLength > 32 on IPv4")
	}
}

func TestBuilderDedup(t *testing.T) {
	b := NewBuilder()
	v := vrp("10.0.0.0/16", 24, 500)
	_ = b.Add(v)
	_ = b.Add(v)
	if got := b.Build().Size(); got != 1 {
		t.Errorf("Size after duplicate add = %d, want 1", got)
	}
}
