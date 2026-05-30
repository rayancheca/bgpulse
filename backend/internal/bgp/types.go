// Package bgp holds the lowest-level domain types shared by every stage of the
// BGPulse pipeline. It depends on nothing else inside the project: the ingestor,
// classifier, RPKI validator, aggregator and API all speak in these types.
//
// The normalized unit of work is [UpdateEvent] (produced by any [Source]); the
// classifier enriches it into an immutable [ClassifiedEvent]. Both are passed by
// value through the pipeline so no stage can mutate another stage's data.
package bgp

import (
	"net/netip"
	"time"
)

// EventID is a monotonic, deterministic identifier assigned by the Source,
// formatted as "<sourceTag>-<seq>" (e.g. "synth-000123"). It is stable across
// runs for a given seed in demo mode.
type EventID string

// UpdateKind discriminates a reachability announcement from a withdrawal.
type UpdateKind uint8

const (
	// KindAnnounce is an UPDATE announcing reachability for a prefix via an AS_PATH.
	KindAnnounce UpdateKind = iota
	// KindWithdraw is a WITHDRAW removing reachability for a prefix (no path).
	KindWithdraw
)

// String returns the lowercase wire token for the kind.
func (k UpdateKind) String() string {
	switch k {
	case KindAnnounce:
		return "announce"
	case KindWithdraw:
		return "withdraw"
	default:
		return "unknown"
	}
}

// RelStatus is the business relationship of AS a toward AS b, as returned by a
// relationship store's Lookup(a, b). It is directional: RelCustomer means b is
// a's customer (a provides transit to b, so the link a->b is "downhill"), while
// RelProvider means b is a's provider (a buys transit from b, so a->b is "uphill").
type RelStatus uint8

const (
	// RelUnknown means the pair is absent from the relationship dataset.
	RelUnknown RelStatus = iota
	// RelCustomer means b is a's customer (a->b is provider-to-customer, downhill).
	RelCustomer
	// RelProvider means b is a's provider (a->b is customer-to-provider, uphill).
	RelProvider
	// RelPeer means a and b are settlement-free peers.
	RelPeer
	// RelSibling means a and b belong to the same organization (phase-transparent).
	RelSibling
)

// String returns the lowercase wire token for the relationship.
func (r RelStatus) String() string {
	switch r {
	case RelCustomer:
		return "customer"
	case RelProvider:
		return "provider"
	case RelPeer:
		return "peer"
	case RelSibling:
		return "sibling"
	default:
		return "unknown"
	}
}

// Invert returns the relationship seen from b's perspective toward a. Customer
// and provider swap; peer, sibling and unknown are self-inverse. This lets a
// relationship store keep one direction per pair and derive the other on lookup.
func (r RelStatus) Invert() RelStatus {
	switch r {
	case RelCustomer:
		return RelProvider
	case RelProvider:
		return RelCustomer
	default:
		return r
	}
}

// VFStatus is the overall security classification of an announced path: the
// combination of the Gao-Rexford valley-free verdict and RPKI origin validation,
// with hijack taking precedence over leak.
type VFStatus uint8

const (
	// VFValid means valley-free and not RPKI-Invalid.
	VFValid VFStatus = iota
	// VFLeak means the AS_PATH violates the valley-free constraint.
	VFLeak
	// VFHijack means the origin is RPKI-Invalid (or a more-specific takeover).
	VFHijack
	// VFUnknown means there was insufficient relationship data to decide, and the
	// path was not a leak or hijack.
	VFUnknown
)

// String returns the lowercase wire token for the status.
func (v VFStatus) String() string {
	switch v {
	case VFValid:
		return "valid"
	case VFLeak:
		return "leak"
	case VFHijack:
		return "hijack"
	default:
		return "unknown"
	}
}

// Severity ranks statuses so the aggregator can keep the "worst" status seen on
// an edge: hijack > leak > valid > unknown.
func (v VFStatus) Severity() int {
	switch v {
	case VFHijack:
		return 3
	case VFLeak:
		return 2
	case VFValid:
		return 1
	default:
		return 0
	}
}

// RPKIStatus is the RFC 6811 Route Origin Validation result for a (prefix, origin)
// pair.
type RPKIStatus uint8

const (
	// RPKINotFound means no VRP covers the announced prefix.
	RPKINotFound RPKIStatus = iota
	// RPKIValid means a covering VRP matches the origin within its maxLength.
	RPKIValid
	// RPKIInvalid means a covering VRP exists but none matches the origin/maxLength.
	RPKIInvalid
)

// String returns the lowercase wire token for the RPKI status.
func (s RPKIStatus) String() string {
	switch s {
	case RPKIValid:
		return "valid"
	case RPKIInvalid:
		return "invalid"
	default:
		return "notfound"
	}
}

// Community is a standard RFC 1997 32-bit BGP community split into its two
// 16-bit halves (conventionally ASN:value).
type Community struct {
	ASN   uint16
	Value uint16
}

// PathHop annotates one directed adjacency on the AS_PATH after classification.
// From is the AS nearer the collector, To the AS nearer the origin — i.e. the
// hops are in wire order so a frontend can index its wire-order asPath directly.
type PathHop struct {
	From       uint32    // AS toward the collector
	To         uint32    // AS toward the origin
	Rel        RelStatus // inferred relationship From -> To
	IsOffender bool      // true if the valley-free constraint broke at this hop
}

// UpdateEvent is the normalized unit flowing through the pipeline, produced by
// any Source. It is immutable after construction: stages never mutate its fields,
// and the slices are never appended to once the event is sent.
type UpdateEvent struct {
	ID          EventID      // assigned by the Source; unique within a run
	Seq         uint64       // monotonic sequence number within a run
	Timestamp   time.Time    // event time (replay: from MRT; demo: deterministic virtual clock)
	Kind        UpdateKind   // announce | withdraw
	Prefix      netip.Prefix // the NLRI prefix (IPv4 or IPv6), masked to its bits
	PeerAS      uint32       // the AS we received this UPDATE from (collector peer)
	ASPath      []uint32     // wire order: index 0 nearest collector, last element = origin
	HasASSet    bool         // true if the original AS_PATH contained an AS_SET segment
	NextHop     netip.Addr   // NEXT_HOP attribute (zero value for withdraws)
	Communities []Community  // standard communities (empty for withdraws)
	OriginAS    uint32       // last element of ASPath (0 if empty / withdraw / AS_SET origin)
}

// ClassifiedEvent wraps an UpdateEvent with its valley-free and RPKI verdicts.
// It embeds the original event by value, preserving immutability across stages.
type ClassifiedEvent struct {
	Event      UpdateEvent // the original normalized event (immutable)
	VFStatus   VFStatus    // overall security verdict (hijack > leak > valid)
	RPKIStatus RPKIStatus  // origin validation verdict
	Hops       []PathHop   // per-adjacency relationship + offender flag (wire order)
	OffenderAS uint32      // the AS at the valley apex (0 if none)
	Reason     string      // short human-readable explanation ("" if normal)
}
