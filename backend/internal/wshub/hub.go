// Package wshub fans classified-event frames out to connected WebSocket clients. A
// single goroutine owns the client set; each client has a bounded send buffer with a
// drop-oldest policy and is disconnected if it falls too far behind, so one slow
// browser can never stall the pipeline. Every client receives a full snapshot on
// connect, making per-event frames advisory and reconnect self-healing.
package wshub

import (
	"context"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
)

const (
	clientSendBuf       = 64
	hubBroadcastBuf     = 256
	clientDropThreshold = 512
	writeTimeout        = 10 * time.Second
)

// Hub owns the set of connected clients and broadcasts frames to them.
type Hub struct {
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
	done       chan struct{}
	clients    map[*Client]struct{}
	snapshot   func() []byte
	log        *slog.Logger
	count      atomic.Int32
}

// New builds a hub. snapshot, if non-nil, produces the frame sent to each client on
// connect.
func New(snapshot func() []byte, log *slog.Logger) *Hub {
	return &Hub{
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte, hubBroadcastBuf),
		done:       make(chan struct{}),
		clients:    make(map[*Client]struct{}),
		snapshot:   snapshot,
		log:        log,
	}
}

// Run owns the client map until ctx is cancelled. It is the only goroutine that
// mutates clients, so no locking is needed.
func (h *Hub) Run(ctx context.Context) {
	defer close(h.done)
	for {
		select {
		case <-ctx.Done():
			for c := range h.clients {
				close(c.send)
				delete(h.clients, c)
			}
			return
		case c := <-h.register:
			h.clients[c] = struct{}{}
			h.count.Add(1)
		case c := <-h.unregister:
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.send)
				h.count.Add(-1)
			}
		case msg := <-h.broadcast:
			for c := range h.clients {
				h.sendToClient(c, msg)
			}
		}
	}
}

// sendToClient enqueues msg, dropping the oldest queued frame when the client's
// buffer is full and disconnecting a client that falls persistently behind. Runs
// only on the Run goroutine.
func (h *Hub) sendToClient(c *Client, msg []byte) {
	select {
	case c.send <- msg:
		return
	default:
	}
	select { // drop the oldest queued frame
	case <-c.send:
	default:
	}
	select { // enqueue the newest
	case c.send <- msg:
	default:
	}
	if c.dropped.Add(1) > clientDropThreshold {
		h.log.Warn("disconnecting slow websocket client", "dropped", c.dropped.Load())
		delete(h.clients, c)
		close(c.send)
		h.count.Add(-1)
	}
}

// Broadcast queues a frame for all clients without ever blocking the caller.
func (h *Hub) Broadcast(msg []byte) {
	select {
	case h.broadcast <- msg:
	default: // hub backlog full; drop (clients re-sync via snapshot)
	}
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int { return int(h.count.Load()) }

// Handler upgrades an HTTP request to a WebSocket, sends the snapshot, and serves
// the connection until it closes.
func (h *Hub) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			h.log.Debug("websocket accept failed", "err", err)
			return
		}
		c := &Client{conn: conn, send: make(chan []byte, clientSendBuf)}

		// Write the snapshot directly before any pump starts, so nothing races on
		// the send channel.
		if h.snapshot != nil {
			if snap := h.snapshot(); len(snap) > 0 {
				ctx, cancel := context.WithTimeout(r.Context(), writeTimeout)
				err := conn.Write(ctx, websocket.MessageText, snap)
				cancel()
				if err != nil {
					conn.CloseNow()
					return
				}
			}
		}

		select {
		case h.register <- c:
		case <-h.done:
			conn.CloseNow()
			return
		}
		go c.writePump()
		c.readPump(r.Context(), h)
	}
}
