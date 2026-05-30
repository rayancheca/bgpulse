package wshub

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

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

func TestSendToClientDropOldestAndDisconnect(t *testing.T) {
	h := New(nil, discardLogger())
	c := &Client{send: make(chan []byte, 2)} // tiny buffer, no conn needed for this path
	h.clients[c] = struct{}{}
	h.count.Store(1)

	h.sendToClient(c, []byte("a"))
	h.sendToClient(c, []byte("b"))
	if len(c.send) != 2 {
		t.Fatalf("buffer should be full, got %d", len(c.send))
	}
	h.sendToClient(c, []byte("c")) // full -> drop oldest
	if c.dropped.Load() != 1 {
		t.Errorf("dropped = %d, want 1", c.dropped.Load())
	}

	for i := 0; i < clientDropThreshold+5; i++ {
		if _, ok := h.clients[c]; !ok {
			break // disconnected; sending on the closed channel would panic
		}
		h.sendToClient(c, []byte("x"))
	}
	if _, ok := h.clients[c]; ok {
		t.Error("a persistently slow client should be disconnected")
	}
	if h.ClientCount() != 0 {
		t.Errorf("ClientCount = %d, want 0", h.ClientCount())
	}
}

func TestHubSnapshotAndBroadcast(t *testing.T) {
	snap := []byte(`{"type":"snapshot"}`)
	h := New(func() []byte { return snap }, discardLogger())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)

	srv := httptest.NewServer(h.Handler())
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	dialCtx, dcancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer dcancel()
	conn, _, err := websocket.Dial(dialCtx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.CloseNow()

	// The first frame is the snapshot.
	_, data, err := conn.Read(dialCtx)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	if string(data) != string(snap) {
		t.Errorf("first frame = %q, want snapshot %q", data, snap)
	}

	// Once registered, a broadcast reaches the client.
	waitFor(t, func() bool { return h.ClientCount() == 1 })
	frame := []byte(`{"type":"event"}`)
	h.Broadcast(frame)
	_, data, err = conn.Read(dialCtx)
	if err != nil {
		t.Fatalf("read broadcast: %v", err)
	}
	if string(data) != string(frame) {
		t.Errorf("broadcast frame = %q, want %q", data, frame)
	}
}
