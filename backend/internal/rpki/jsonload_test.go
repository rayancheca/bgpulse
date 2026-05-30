package rpki

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/rayancheca/bgpulse/backend/internal/bgp"
)

func TestLoadVRPsJSON(t *testing.T) {
	input := `{
	  "roas": [
	    {"prefix": "10.0.0.0/16", "maxLength": 24, "asn": "AS500"},
	    {"prefix": "192.0.2.0/24", "asn": 64500},
	    {"prefix": "2001:db8::/32", "maxLength": 48, "asn": "AS500"}
	  ]
	}`
	store, err := LoadVRPsJSON(strings.NewReader(input))
	if err != nil {
		t.Fatalf("LoadVRPsJSON: %v", err)
	}
	if store.Size() != 3 {
		t.Fatalf("Size = %d, want 3", store.Size())
	}

	// "AS500" string form parsed, within maxLength -> Valid.
	if s := store.Validate(netip.MustParsePrefix("10.0.0.0/24"), 500).Status; s != bgp.RPKIValid {
		t.Errorf("10.0.0.0/24 by AS500 = %v, want valid", s)
	}
	// Numeric asn form, default maxLength == prefix length (24) -> exact match Valid.
	if s := store.Validate(netip.MustParsePrefix("192.0.2.0/24"), 64500).Status; s != bgp.RPKIValid {
		t.Errorf("192.0.2.0/24 by 64500 = %v, want valid", s)
	}
	// Default maxLength means a more-specific is Invalid (covered, exceeds maxLength).
	if s := store.Validate(netip.MustParsePrefix("192.0.2.0/25"), 64500).Status; s != bgp.RPKIInvalid {
		t.Errorf("192.0.2.0/25 by 64500 = %v, want invalid (default maxLength)", s)
	}
	// IPv6 valid.
	if s := store.Validate(netip.MustParsePrefix("2001:db8:1::/48"), 500).Status; s != bgp.RPKIValid {
		t.Errorf("2001:db8:1::/48 by AS500 = %v, want valid", s)
	}
}

func TestLoadVRPsJSONErrors(t *testing.T) {
	cases := map[string]string{
		"bad prefix":        `{"roas":[{"prefix":"10.0.0.0/99","asn":"AS1"}]}`,
		"maxLength too big": `{"roas":[{"prefix":"10.0.0.0/24","maxLength":40,"asn":"AS1"}]}`,
		"bad asn string":    `{"roas":[{"prefix":"10.0.0.0/24","asn":"ASxyz"}]}`,
		"malformed json":    `{"roas":[`,
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadVRPsJSON(strings.NewReader(input)); err == nil {
				t.Errorf("expected error for %q", input)
			}
		})
	}
}
