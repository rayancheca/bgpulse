package wshub

import (
	"context"
	"sync/atomic"

	"github.com/coder/websocket"
)

// Client is one connected WebSocket peer with a bounded send buffer.
type Client struct {
	conn    *websocket.Conn
	send    chan []byte
	dropped atomic.Int32
}

// writePump drains the send buffer to the connection. It exits when the hub closes
// the send channel (disconnect or shutdown) or a write fails.
func (c *Client) writePump() {
	for msg := range c.send {
		ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
		err := c.conn.Write(ctx, websocket.MessageText, msg)
		cancel()
		if err != nil {
			break
		}
	}
	c.conn.CloseNow()
}

// readPump discards inbound frames (the protocol is server->client only) and exists
// solely to detect disconnect, after which it unregisters the client.
func (c *Client) readPump(ctx context.Context, h *Hub) {
	defer func() {
		select {
		case h.unregister <- c:
		case <-h.done:
		}
	}()
	for {
		if _, _, err := c.conn.Read(ctx); err != nil {
			return
		}
	}
}
