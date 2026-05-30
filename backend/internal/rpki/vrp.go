// Package rpki implements RFC 6811 BGP Prefix Origin Validation against a set of
// Validated ROA Payloads (VRPs). The crucial subtlety it gets right: a VRP
// "covers" an announced prefix purely by containment (the VRP prefix is a
// less-specific-or-equal of the announcement). maxLength is part of the *match*
// test, not the cover test — so a prefix contained by a VRP but more specific than
// the VRP's maxLength, with no matching VRP, is Invalid, not NotFound. That is the
// rule that catches more-specific prefix hijacks.
package rpki

import (
	"net/netip"

	"github.com/rayancheca/bgpulse/backend/internal/bgp"
)

// VRP is a Validated ROA Payload: an authorization that OriginAS may originate
// Prefix at lengths up to MaxLength. OriginAS == 0 (AS0) is a disavowal that can
// never produce a Valid result.
type VRP struct {
	Prefix    netip.Prefix // masked to its bits on construction
	MaxLength uint8        // >= Prefix.Bits(), <= family max (32 v4 / 128 v6)
	OriginAS  uint32
}

// Result is the outcome of validating one announced (prefix, origin) pair.
type Result struct {
	Status       bgp.RPKIStatus // notfound | valid | invalid
	MatchedVRP   *VRP           // the VRP that produced Valid (nil otherwise)
	CoveringVRPs int            // number of covering VRPs (explains an Invalid)
}
