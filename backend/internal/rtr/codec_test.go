package rtr

import (
	"bytes"
	"net/netip"
	"testing"
)

func TestPrefixPDURoundTrip(t *testing.T) {
	cases := []struct {
		name     string
		announce bool
		cidr     string
		maxLen   uint8
		asn      uint32
		wantV6   bool
	}{
		{"v4 announce", true, "10.0.0.0/16", 24, 65001, false},
		{"v4 withdraw", false, "192.0.2.0/24", 24, 0, false},
		{"v6 announce", true, "2001:db8::/32", 48, 500, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prefix := netip.MustParsePrefix(tc.cidr)
			raw := EncodePrefix(tc.announce, prefix, tc.maxLen, tc.asn)
			pdu, err := readPDU(bytes.NewReader(raw))
			if err != nil {
				t.Fatalf("readPDU: %v", err)
			}
			p, ok := pdu.(PrefixPDU)
			if !ok {
				t.Fatalf("got %T, want PrefixPDU", pdu)
			}
			if p.Announce != tc.announce || p.Prefix != prefix.Masked() || p.MaxLen != tc.maxLen || p.ASN != tc.asn || p.IsV6 != tc.wantV6 {
				t.Errorf("round-trip mismatch: %+v", p)
			}
		})
	}
}

func TestControlPDURoundTrips(t *testing.T) {
	// End Of Data
	eod, err := readPDU(bytes.NewReader(EncodeEndOfData(7, 42, 3600, 600, 7200)))
	if err != nil {
		t.Fatalf("EndOfData readPDU: %v", err)
	}
	if e, ok := eod.(EndOfData); !ok || e.Serial != 42 || e.Refresh != 3600 || e.Session != 7 {
		t.Errorf("EndOfData round-trip = %+v", eod)
	}

	// Serial Notify
	sn, _ := readPDU(bytes.NewReader(EncodeSerialNotify(7, 99)))
	if s, ok := sn.(SerialNotify); !ok || s.Serial != 99 || s.Session != 7 {
		t.Errorf("SerialNotify round-trip = %+v", sn)
	}

	// Cache Response / Cache Reset
	if cr, _ := readPDU(bytes.NewReader(EncodeCacheResponse(3))); func() bool { _, ok := cr.(CacheResponse); return !ok }() {
		t.Error("CacheResponse did not round-trip")
	}
	if cr, _ := readPDU(bytes.NewReader(EncodeCacheReset())); func() bool { _, ok := cr.(CacheReset); return !ok }() {
		t.Error("CacheReset did not round-trip")
	}

	// Client-originated PDUs the cache reads.
	var buf bytes.Buffer
	if err := writeResetQuery(&buf); err != nil {
		t.Fatalf("writeResetQuery: %v", err)
	}
	if rq, _ := readPDU(&buf); func() bool { _, ok := rq.(ResetQuery); return !ok }() {
		t.Error("ResetQuery did not round-trip")
	}
	buf.Reset()
	if err := writeSerialQuery(&buf, 7, 5); err != nil {
		t.Fatalf("writeSerialQuery: %v", err)
	}
	if sq, _ := readPDU(&buf); func() bool { s, ok := sq.(SerialQuery); return !ok || s.Serial != 5 || s.Session != 7 }() {
		t.Error("SerialQuery did not round-trip")
	}
}
