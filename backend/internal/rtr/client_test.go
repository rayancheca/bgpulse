package rtr

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/rayancheca/bgpulse/backend/internal/bgp"
	"github.com/rayancheca/bgpulse/backend/internal/rpki"
)

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("condition not met within timeout")
}

func mustWrite(t *testing.T, w io.Writer, b []byte) {
	t.Helper()
	if _, err := w.Write(b); err != nil {
		t.Fatalf("server write: %v", err)
	}
}

// TestRTRClientFullSyncAndDelta drives the real Client.Run against an in-memory fake
// RTR cache over net.Pipe, exercising the Reset Query full sync and a Serial
// Notify-driven incremental delta.
func TestRTRClientFullSyncAndDelta(t *testing.T) {
	cConn, sConn := net.Pipe()
	live := rpki.NewLive(nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := NewClient("pipe", live, logger).
		WithDialer(func(context.Context) (net.Conn, error) { return cConn, nil })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { _ = client.Run(ctx); close(done) }()

	const sid = uint16(7)

	// The client opens with a Reset Query.
	if pdu, err := readPDU(sConn); err != nil {
		t.Fatalf("read ResetQuery: %v", err)
	} else if _, ok := pdu.(ResetQuery); !ok {
		t.Fatalf("first PDU = %T, want ResetQuery", pdu)
	}

	// Full table: 3 VRPs.
	mustWrite(t, sConn, EncodeCacheResponse(sid))
	mustWrite(t, sConn, EncodePrefix(true, netip.MustParsePrefix("10.0.0.0/16"), 24, 500))
	mustWrite(t, sConn, EncodePrefix(true, netip.MustParsePrefix("192.0.2.0/24"), 24, 0))
	mustWrite(t, sConn, EncodePrefix(true, netip.MustParsePrefix("2001:db8::/32"), 48, 500))
	mustWrite(t, sConn, EncodeEndOfData(sid, 1, 3600, 600, 7200))

	waitFor(t, func() bool { return live.Size() == 3 })
	if got := live.Validate(netip.MustParsePrefix("10.0.0.0/24"), 500).Status; got != bgp.RPKIValid {
		t.Errorf("after full sync 10.0.0.0/24 by 500 = %v, want valid", got)
	}

	// Incremental: notify, expect a Serial Query, send a delta (withdraw + announce).
	mustWrite(t, sConn, EncodeSerialNotify(sid, 2))
	if pdu, err := readPDU(sConn); err != nil {
		t.Fatalf("read SerialQuery: %v", err)
	} else if sq, ok := pdu.(SerialQuery); !ok || sq.Serial != 1 {
		t.Fatalf("expected SerialQuery serial=1, got %#v", pdu)
	}
	mustWrite(t, sConn, EncodeCacheResponse(sid))
	mustWrite(t, sConn, EncodePrefix(false, netip.MustParsePrefix("10.0.0.0/16"), 24, 500)) // withdraw
	mustWrite(t, sConn, EncodePrefix(true, netip.MustParsePrefix("30.0.0.0/8"), 16, 700))   // announce
	mustWrite(t, sConn, EncodeEndOfData(sid, 2, 3600, 600, 7200))

	waitFor(t, func() bool {
		return live.Size() == 3 &&
			live.Validate(netip.MustParsePrefix("30.0.0.0/8"), 700).Status == bgp.RPKIValid
	})
	if got := live.Validate(netip.MustParsePrefix("10.0.0.0/24"), 500).Status; got != bgp.RPKINotFound {
		t.Errorf("after delta withdrawal 10.0.0.0/24 = %v, want notfound", got)
	}

	cancel()
	_ = sConn.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("client did not stop after cancel")
	}
}
