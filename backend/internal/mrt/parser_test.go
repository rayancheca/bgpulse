package mrt

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/rayancheca/bgpulse/backend/internal/bgp"
)

func TestDecodeSampleMRT(t *testing.T) {
	data, err := BuildSampleMRT()
	if err != nil {
		t.Fatalf("BuildSampleMRT: %v", err)
	}
	events, err := DecodeStream(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("DecodeStream: %v", err)
	}
	if len(events) != 4 {
		t.Fatalf("got %d events, want 4: %+v", len(events), events)
	}

	// 1) Normal IPv4 announce.
	a := events[0]
	if a.Kind != bgp.KindAnnounce || a.Prefix != netip.MustParsePrefix("10.0.0.0/24") {
		t.Errorf("event0 = %v %s, want announce 10.0.0.0/24", a.Kind, a.Prefix)
	}
	if !equalU32(a.ASPath, []uint32{65001, 3356, 174}) {
		t.Errorf("event0 ASPath = %v, want [65001 3356 174]", a.ASPath)
	}
	if a.OriginAS != 174 || a.PeerAS != 65001 {
		t.Errorf("event0 origin/peer = %d/%d, want 174/65001", a.OriginAS, a.PeerAS)
	}
	if a.HasASSet {
		t.Error("event0 should not be flagged AS_SET")
	}
	if len(a.Communities) != 1 || a.Communities[0] != (bgp.Community{ASN: 2604, Value: 100}) {
		t.Errorf("event0 communities = %v, want [2604:100]", a.Communities)
	}
	if a.NextHop != netip.MustParseAddr("192.0.2.1") {
		t.Errorf("event0 nextHop = %v, want 192.0.2.1", a.NextHop)
	}

	// 2) AS_SET-aggregated announce: trailing set collapses to one hop, origin unverifiable.
	b := events[1]
	if b.Prefix != netip.MustParsePrefix("20.0.0.0/16") || !b.HasASSet {
		t.Errorf("event1 = %s hasASSet=%v, want 20.0.0.0/16 hasASSet=true", b.Prefix, b.HasASSet)
	}
	if b.OriginAS != 0 {
		t.Errorf("event1 origin = %d, want 0 (AS_SET origin is unverifiable)", b.OriginAS)
	}
	if !equalU32(b.ASPath, []uint32{65002, 3356, 64500}) {
		t.Errorf("event1 ASPath = %v, want [65002 3356 64500] (set collapsed to first member)", b.ASPath)
	}

	// 3) IPv6 MP_REACH announce.
	c := events[2]
	if c.Prefix != netip.MustParsePrefix("2001:db8::/32") || c.OriginAS != 6939 {
		t.Errorf("event2 = %s origin %d, want 2001:db8::/32 origin 6939", c.Prefix, c.OriginAS)
	}
	if c.NextHop != netip.MustParseAddr("2001:db8::1") {
		t.Errorf("event2 nextHop = %v, want 2001:db8::1", c.NextHop)
	}

	// 4) IPv4 withdrawal.
	d := events[3]
	if d.Kind != bgp.KindWithdraw || d.Prefix != netip.MustParsePrefix("10.0.0.0/24") {
		t.Errorf("event3 = %v %s, want withdraw 10.0.0.0/24", d.Kind, d.Prefix)
	}
	if d.PeerAS != 65001 {
		t.Errorf("event3 peer = %d, want 65001", d.PeerAS)
	}
}

func TestDecodeEmptyStream(t *testing.T) {
	events, err := DecodeStream(bytes.NewReader(nil))
	if err != nil {
		t.Fatalf("DecodeStream(empty): %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events from empty stream, got %d", len(events))
	}
}

func TestReplaySourceFromBytesAssignsIDs(t *testing.T) {
	data, err := BuildSampleMRT()
	if err != nil {
		t.Fatalf("BuildSampleMRT: %v", err)
	}
	src, err := NewReplaySourceFromBytes(data, 0, false)
	if err != nil {
		t.Fatalf("NewReplaySourceFromBytes: %v", err)
	}
	if src.Tag() != "mrt" {
		t.Errorf("Tag = %q, want mrt", src.Tag())
	}
	if src.Count() != 4 {
		t.Fatalf("Count = %d, want 4", src.Count())
	}
}

func equalU32(a, b []uint32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
