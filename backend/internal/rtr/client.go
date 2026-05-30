package rtr

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/rayancheca/bgpulse/backend/internal/rpki"
)

const (
	defaultRefresh = time.Hour      // fallback poll interval until the cache supplies one
	initialBackoff = time.Second    // reconnect backoff floor
	maxBackoff     = 30 * time.Second
)

// errFatal marks an unrecoverable RTR error (e.g. an unsupported protocol version).
var errFatal = errors.New("rtr: fatal error")

func vrpKey(v rpki.VRP) string {
	return fmt.Sprintf("%s|%d|%d", v.Prefix, v.MaxLength, v.OriginAS)
}

// Client maintains one RTR session against an RPKI cache and keeps an rpki.Live
// store in sync with it.
type Client struct {
	dial    func(ctx context.Context) (net.Conn, error)
	live    *rpki.Live
	log     *slog.Logger
	refresh time.Duration

	sessionID uint16
	serial    uint32
	hasSerial bool
	current   map[string]rpki.VRP
}

// NewClient builds an RTR client that dials addr ("host:port") over TCP.
func NewClient(addr string, live *rpki.Live, log *slog.Logger) *Client {
	return &Client{
		dial: func(ctx context.Context) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "tcp", addr)
		},
		live:    live,
		log:     log,
		refresh: defaultRefresh,
		current: map[string]rpki.VRP{},
	}
}

// WithDialer overrides the dialer; used by tests with net.Pipe.
func (c *Client) WithDialer(d func(ctx context.Context) (net.Conn, error)) *Client {
	c.dial = d
	return c
}

// Run connects and maintains the session until ctx is cancelled, reconnecting with
// exponential backoff on transient errors and stopping on fatal ones.
func (c *Client) Run(ctx context.Context) error {
	backoff := initialBackoff
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err := c.session(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errors.Is(err, errFatal) {
			c.log.Error("rtr fatal error, giving up", "err", err)
			return err
		}
		c.log.Warn("rtr session ended; reconnecting", "err", err, "backoff", backoff.String())
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return ctx.Err()
		}
		if backoff < maxBackoff {
			backoff *= 2
		}
	}
}

type changeRec struct {
	vrp      rpki.VRP
	announce bool
}

// session runs one connection: dial, full sync via Reset Query, then incremental
// updates driven by Serial Notify or the refresh timer.
func (c *Client) session(ctx context.Context) error {
	conn, err := c.dial(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()

	if err := writeResetQuery(conn); err != nil {
		return err
	}
	fullSync := true
	var pending []changeRec

	for {
		if c.refresh > 0 {
			_ = conn.SetReadDeadline(time.Now().Add(c.refresh))
		}
		pdu, err := readPDU(conn)
		if err != nil {
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() && c.hasSerial {
				if e := writeSerialQuery(conn, c.sessionID, c.serial); e != nil {
					return e
				}
				fullSync, pending = false, pending[:0]
				continue
			}
			return err
		}

		switch p := pdu.(type) {
		case CacheResponse:
			c.sessionID = p.Session
			pending = pending[:0]
		case PrefixPDU:
			pending = append(pending, changeRec{vrp: toVRP(p), announce: p.Announce})
		case EndOfData:
			c.commit(fullSync, pending)
			c.sessionID, c.serial, c.hasSerial = p.Session, p.Serial, true
			if p.Refresh > 0 {
				c.refresh = time.Duration(p.Refresh) * time.Second
			}
			fullSync, pending = false, pending[:0]
			c.log.Info("rtr synced", "serial", c.serial, "vrps", c.live.Size())
		case SerialNotify:
			if e := writeSerialQuery(conn, c.sessionID, c.serial); e != nil {
				return e
			}
			fullSync, pending = false, pending[:0]
		case CacheReset:
			c.current = map[string]rpki.VRP{}
			if e := writeResetQuery(conn); e != nil {
				return e
			}
			fullSync, pending = true, pending[:0]
		case ErrorReport:
			if p.Code == ErrUnsupportedProtocolVersion {
				return fmt.Errorf("%w: %s", errFatal, p.Text)
			}
			return fmt.Errorf("rtr cache error %d: %s", p.Code, p.Text)
		case RouterKey:
			// BGPsec router keys are out of scope; ignore.
		}
	}
}

func toVRP(p PrefixPDU) rpki.VRP {
	return rpki.VRP{Prefix: p.Prefix, MaxLength: p.MaxLen, OriginAS: p.ASN}
}

// commit applies the accumulated changes to the current VRP set and swaps a freshly
// built immutable store into the live validator.
func (c *Client) commit(fullSync bool, changes []changeRec) {
	if fullSync {
		c.current = make(map[string]rpki.VRP, len(changes))
	}
	for _, ch := range changes {
		k := vrpKey(ch.vrp)
		if ch.announce {
			c.current[k] = ch.vrp
		} else {
			delete(c.current, k)
		}
	}
	b := rpki.NewBuilder()
	for _, v := range c.current {
		_ = b.Add(v)
	}
	c.live.Replace(b.Build())
}
