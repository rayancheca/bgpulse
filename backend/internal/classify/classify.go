// Package classify combines the Gao-Rexford valley-free verdict and RFC 6811 RPKI
// origin validation into the single security classification carried by every
// ClassifiedEvent. Precedence is Hijack > Leak > Normal: an unauthorized origin is
// the more severe, more actionable signal, but both sub-results are retained so the
// UI can say "hijack — also leaks".
package classify

import (
	"fmt"
	"net/netip"

	"github.com/rayancheca/bgpulse/backend/internal/bgp"
	"github.com/rayancheca/bgpulse/backend/internal/rpki"
	"github.com/rayancheca/bgpulse/backend/internal/valleyfree"
)

// Validator performs RPKI origin validation. *rpki.VRPStore satisfies it; a live
// RTR-backed store can swap implementations behind the same interface.
type Validator interface {
	Validate(prefix netip.Prefix, origin uint32) rpki.Result
}

// Classifier is the concrete bgp.Classifier. The relationship oracle and validator
// are injected once and read-only while streaming, so Classify is safe to call
// repeatedly and is pure with respect to the event.
type Classifier struct {
	rel valleyfree.RelLookup
	vrp Validator
}

// New builds a Classifier from a relationship oracle and an RPKI validator.
func New(rel valleyfree.RelLookup, vrp Validator) *Classifier {
	return &Classifier{rel: rel, vrp: vrp}
}

// Classify enriches an UpdateEvent into a ClassifiedEvent. Withdraws carry no path
// and are returned neutral. For announcements it runs the valley-free walk and RPKI
// validation, then resolves the combined status by precedence.
func (c *Classifier) Classify(ev bgp.UpdateEvent) bgp.ClassifiedEvent {
	out := bgp.ClassifiedEvent{Event: ev, VFStatus: bgp.VFValid, RPKIStatus: bgp.RPKINotFound}
	if ev.Kind == bgp.KindWithdraw {
		return out
	}

	vf := valleyfree.ClassifyPath(ev.ASPath, ev.HasASSet, c.rel)
	out.Hops = vf.Hops

	// Origin is known only when it is a real ASN (AS_SET / empty-path origins are 0).
	originKnown := ev.OriginAS != 0
	if originKnown {
		out.RPKIStatus = c.vrp.Validate(ev.Prefix, ev.OriginAS).Status
	}

	switch {
	case originKnown && out.RPKIStatus == bgp.RPKIInvalid:
		out.VFStatus = bgp.VFHijack
		out.OffenderAS = ev.OriginAS
		out.Reason = fmt.Sprintf("origin AS%d not authorized to announce %s (RPKI Invalid)",
			ev.OriginAS, ev.Prefix)
		if vf.IsLeak {
			out.Reason += "; path also leaks: " + vf.Reason
		}
	case vf.IsLeak:
		out.VFStatus = bgp.VFLeak
		out.OffenderAS = vf.OffenderAS
		out.Reason = vf.Reason
	case len(ev.ASPath) > 1 && vf.KnownHops == 0:
		// We saw inter-AS hops but had no relationship data for any of them.
		out.VFStatus = bgp.VFUnknown
	default:
		out.VFStatus = bgp.VFValid
	}
	return out
}
