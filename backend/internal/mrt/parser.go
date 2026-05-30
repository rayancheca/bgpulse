// Package mrt parses RouteViews / RIPE RIS MRT dump files into the normalized
// UpdateEvent stream the rest of BGPulse consumes. The byte-level MRT/BGP wire
// decode is delegated to gobgp v4 (a production decoder that correctly handles the
// 2-byte/4-byte ASN ambiguity, AS_TRANS, MP_REACH, and record framing); this
// package owns the normalization above it: flattening AS_PATH segments, collapsing
// AS_SET aggregation to a single opaque hop, resolving the origin, and fanning a
// multi-NLRI UPDATE into one UpdateEvent per prefix.
package mrt

import (
	"fmt"
	"net/netip"
	"time"

	gobgp "github.com/osrg/gobgp/v4/pkg/packet/bgp"
	gomrt "github.com/osrg/gobgp/v4/pkg/packet/mrt"

	"github.com/rayancheca/bgpulse/backend/internal/bgp"
)

// recordToEvents converts one MRT record into zero or more normalized UpdateEvents.
// Only BGP4MP MESSAGE records carrying a BGP UPDATE yield events; RIB snapshots,
// state changes, and non-UPDATE BGP messages yield none. ID and Seq are left zero;
// the replay source assigns them on emission.
func recordToEvents(msg *gomrt.MRTMessage) []bgp.UpdateEvent {
	bm, ok := msg.Body.(*gomrt.BGP4MPMessage)
	if !ok || bm.BGPMessage == nil {
		return nil
	}
	upd, ok := bm.BGPMessage.Body.(*gobgp.BGPUpdate)
	if !ok {
		return nil
	}

	ts := time.Unix(int64(msg.Header.Timestamp), 0).UTC()
	asPath, hasASSet, lastWasSet := extractASPath(upd.PathAttributes)
	origin := originAS(asPath, lastWasSet)
	nextHop := extractNextHop(upd.PathAttributes)
	comms := extractCommunities(upd.PathAttributes)
	base := bgp.UpdateEvent{Timestamp: ts, PeerAS: bm.PeerAS}

	var out []bgp.UpdateEvent
	for _, w := range upd.WithdrawnRoutes { // IPv4 withdrawals
		if p, err := nlriPrefix(w.NLRI); err == nil {
			out = append(out, withdrawEvent(base, p))
		}
	}
	for _, n := range upd.NLRI { // IPv4 announcements
		if p, err := nlriPrefix(n.NLRI); err == nil {
			out = append(out, announceEvent(base, p, asPath, hasASSet, origin, nextHop, comms))
		}
	}
	for _, attr := range upd.PathAttributes { // IPv6 via multiprotocol attributes
		switch a := attr.(type) {
		case *gobgp.PathAttributeMpReachNLRI:
			for _, n := range a.Value {
				if p, err := nlriPrefix(n.NLRI); err == nil {
					out = append(out, announceEvent(base, p, asPath, hasASSet, origin, a.Nexthop, comms))
				}
			}
		case *gobgp.PathAttributeMpUnreachNLRI:
			for _, n := range a.Value {
				if p, err := nlriPrefix(n.NLRI); err == nil {
					out = append(out, withdrawEvent(base, p))
				}
			}
		}
	}
	return out
}

func announceEvent(base bgp.UpdateEvent, p netip.Prefix, asPath []uint32, hasASSet bool, origin uint32, nh netip.Addr, comms []bgp.Community) bgp.UpdateEvent {
	ev := base
	ev.Kind = bgp.KindAnnounce
	ev.Prefix = p
	ev.ASPath = asPath
	ev.HasASSet = hasASSet
	ev.OriginAS = origin
	ev.NextHop = nh
	ev.Communities = comms
	return ev
}

func withdrawEvent(base bgp.UpdateEvent, p netip.Prefix) bgp.UpdateEvent {
	ev := base
	ev.Kind = bgp.KindWithdraw
	ev.Prefix = p
	return ev
}

// extractASPath flattens the AS_PATH attribute to wire order. Each AS_SEQUENCE
// contributes its ASNs; each AS_SET is collapsed to a single opaque representative
// hop (its first member) and flags hasASSet. lastWasSet reports whether the final
// segment was an AS_SET (which makes the origin unverifiable).
func extractASPath(attrs []gobgp.PathAttributeInterface) (path []uint32, hasASSet, lastWasSet bool) {
	for _, attr := range attrs {
		ap, ok := attr.(*gobgp.PathAttributeAsPath)
		if !ok {
			continue
		}
		for _, seg := range ap.Value {
			as := seg.GetAS()
			isSet := seg.GetType() == gobgp.BGP_ASPATH_ATTR_TYPE_SET
			lastWasSet = isSet
			if isSet {
				hasASSet = true
				if len(as) > 0 {
					path = append(path, as[0])
				}
				continue
			}
			path = append(path, as...)
		}
		return path, hasASSet, lastWasSet
	}
	return nil, false, false
}

// originAS returns the originating ASN: the last AS_SEQUENCE element. It is 0 when
// the path is empty or ends in an AS_SET (an unverifiable, aggregated origin).
func originAS(path []uint32, lastWasSet bool) uint32 {
	if lastWasSet || len(path) == 0 {
		return 0
	}
	return path[len(path)-1]
}

func extractNextHop(attrs []gobgp.PathAttributeInterface) netip.Addr {
	for _, attr := range attrs {
		if nh, ok := attr.(*gobgp.PathAttributeNextHop); ok {
			return nh.Value
		}
	}
	return netip.Addr{}
}

func extractCommunities(attrs []gobgp.PathAttributeInterface) []bgp.Community {
	for _, attr := range attrs {
		c, ok := attr.(*gobgp.PathAttributeCommunities)
		if !ok {
			continue
		}
		out := make([]bgp.Community, 0, len(c.Value))
		for _, v := range c.Value {
			out = append(out, bgp.Community{ASN: uint16(v >> 16), Value: uint16(v)})
		}
		return out
	}
	return nil
}

// nlriPrefix extracts a masked netip.Prefix from a gobgp NLRI.
func nlriPrefix(n gobgp.NLRI) (netip.Prefix, error) {
	p, err := netip.ParsePrefix(n.String())
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("mrt: bad NLRI %q: %w", n.String(), err)
	}
	return p.Masked(), nil
}
