package rpki

import (
	"encoding/json"
	"fmt"
	"io"
	"net/netip"
	"strconv"
	"strings"
)

// asnJSON decodes an ASN that may be encoded either as a JSON number (rpki-client
// style: 64500) or a JSON string with an "AS" prefix (Routinator style: "AS64500").
type asnJSON uint32

// UnmarshalJSON accepts both the numeric and "AS"-prefixed string ASN encodings.
func (a *asnJSON) UnmarshalJSON(b []byte) error {
	var n uint32
	if err := json.Unmarshal(b, &n); err == nil {
		*a = asnJSON(n)
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("rpki: asn must be a number or string, got %s", string(b))
	}
	s = strings.TrimPrefix(strings.TrimSpace(s), "AS")
	v, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return fmt.Errorf("rpki: bad asn %q: %w", s, err)
	}
	*a = asnJSON(v)
	return nil
}

// roaJSON mirrors one entry of a Routinator/rpki-client JSON export.
type roaJSON struct {
	Prefix    string  `json:"prefix"`
	MaxLength *uint8  `json:"maxLength"`
	ASN       asnJSON `json:"asn"`
}

// exportJSON is the top-level shape of a Routinator/rpki-client VRP export.
type exportJSON struct {
	ROAs []roaJSON `json:"roas"`
}

// LoadVRPsJSON parses a Routinator/rpki-client VRP JSON export and returns an
// immutable VRPStore. A missing maxLength defaults to the prefix length. Malformed
// prefixes, out-of-range maxLength, and bad ASNs are errors annotated with the
// offending entry index; exact-duplicate VRPs are de-duplicated.
func LoadVRPsJSON(r io.Reader) (*VRPStore, error) {
	var export exportJSON
	if err := json.NewDecoder(r).Decode(&export); err != nil {
		return nil, fmt.Errorf("rpki: decode VRP JSON: %w", err)
	}
	b := NewBuilder()
	for i, roa := range export.ROAs {
		p, err := netip.ParsePrefix(roa.Prefix)
		if err != nil {
			return nil, fmt.Errorf("rpki: roa[%d] bad prefix %q: %w", i, roa.Prefix, err)
		}
		p = p.Masked()
		maxLen := uint8(p.Bits())
		if roa.MaxLength != nil {
			maxLen = *roa.MaxLength
		}
		if err := b.Add(VRP{Prefix: p, MaxLength: maxLen, OriginAS: uint32(roa.ASN)}); err != nil {
			return nil, fmt.Errorf("rpki: roa[%d]: %w", i, err)
		}
	}
	return b.Build(), nil
}
