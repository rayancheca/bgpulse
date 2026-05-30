package synth

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net/netip"
	"time"

	"github.com/rayancheca/bgpulse/backend/internal/bgp"
)

// baseTime anchors the deterministic virtual clock. It is a fixed constant so the
// generated timestamps are reproducible across runs (no wall-clock dependence).
var baseTime = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

const (
	genGamma       = 0x9E3779B97F4A7C15 ^ 0xA5A5 // distinct PCG stream from the topology
	recentCap      = 256
	minRecentForWd = 8 // require a backlog before emitting withdraws
)

// GenConfig tunes the synthetic stream.
type GenConfig struct {
	Seed         uint64  // PRNG seed; identical seed => identical stream
	LeakEvery    int     // inject a leak every N events (0 disables)
	HijackEvery  int     // inject a hijack every N events (0 disables)
	WithdrawProb float64 // probability a non-anomaly event is a withdraw
	EventsPerSec float64 // wall-clock pacing for Events(); 0 => as fast as possible
}

// DefaultGenConfig returns the demo defaults: a leak roughly every 12 events, a
// hijack every 19, paced at ~18 events/sec.
func DefaultGenConfig() GenConfig {
	return GenConfig{Seed: DefaultSeed, LeakEvery: 12, HijackEvery: 19, WithdrawProb: 0.08, EventsPerSec: 18}
}

// recentAnn remembers an announcement so it can later be withdrawn.
type recentAnn struct {
	prefix netip.Prefix
	peerAS uint32
}

// Generator is a deterministic synthetic BGP Source. A given (topology, config) seed
// produces a byte-identical sequence of events from Next.
type Generator struct {
	topo   *Topology
	cfg    GenConfig
	rng    *rand.Rand
	seq    uint64
	clock  time.Time
	recent []recentAnn
}

// NewGenerator builds a generator over the given topology.
func NewGenerator(topo *Topology, cfg GenConfig) *Generator {
	return &Generator{
		topo:  topo,
		cfg:   cfg,
		rng:   rand.New(rand.NewPCG(cfg.Seed, cfg.Seed^genGamma)),
		clock: baseTime,
	}
}

// Tag identifies this source for EventID prefixes.
func (g *Generator) Tag() string { return "synth" }

// Err always returns nil: the synthetic source never fails.
func (g *Generator) Err() error { return nil }

// Next produces the next event deterministically, advancing the sequence number and
// the virtual clock. It is not safe for concurrent use.
func (g *Generator) Next() bgp.UpdateEvent {
	g.seq++
	g.clock = g.clock.Add(time.Duration(40+g.rng.IntN(160)) * time.Millisecond)

	var ev bgp.UpdateEvent
	switch {
	case g.cfg.LeakEvery > 0 && g.seq%uint64(g.cfg.LeakEvery) == 0:
		ev = g.makeLeak()
	case g.cfg.HijackEvery > 0 && g.seq%uint64(g.cfg.HijackEvery) == 0:
		ev = g.makeHijack()
	case len(g.recent) >= minRecentForWd && g.rng.Float64() < g.cfg.WithdrawProb:
		ev = g.makeWithdraw()
	default:
		ev = g.makeAnnounce()
	}

	ev.ID = bgp.EventID(fmt.Sprintf("synth-%06d", g.seq))
	ev.Seq = g.seq
	ev.Timestamp = g.clock
	if ev.Kind == bgp.KindAnnounce {
		g.remember(ev)
	}
	return ev
}

// Events spawns a goroutine that emits events on the returned channel until ctx is
// cancelled. With EventsPerSec > 0 it paces emission for a watchable demo; tests use
// Next directly so they run without sleeping.
func (g *Generator) Events(ctx context.Context) <-chan bgp.UpdateEvent {
	out := make(chan bgp.UpdateEvent)
	var interval time.Duration
	if g.cfg.EventsPerSec > 0 {
		interval = time.Duration(float64(time.Second) / g.cfg.EventsPerSec)
	}
	go func() {
		defer close(out)
		for {
			ev := g.Next()
			select {
			case out <- ev:
			case <-ctx.Done():
				return
			}
			if interval <= 0 {
				continue
			}
			select {
			case <-time.After(interval):
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

func (g *Generator) makeAnnounce() bgp.UpdateEvent {
	origin := g.topo.originables[g.rng.IntN(len(g.topo.originables))]
	path := g.topo.validPath(origin, g.rng)
	return g.announce(g.pickPrefix(origin), path)
}

// announce assembles a normalized announce event from a wire-order path.
func (g *Generator) announce(prefix netip.Prefix, wirePath []uint32) bgp.UpdateEvent {
	var origin, peer uint32
	if n := len(wirePath); n > 0 {
		origin = wirePath[n-1]
		peer = wirePath[0]
	}
	return bgp.UpdateEvent{
		Kind:        bgp.KindAnnounce,
		Prefix:      prefix,
		PeerAS:      peer,
		ASPath:      wirePath,
		NextHop:     netip.AddrFrom4([4]byte{10, byte(peer >> 8), byte(peer), 1}),
		Communities: []bgp.Community{{ASN: uint16(origin & 0xffff), Value: 100}},
		OriginAS:    origin,
	}
}

func (g *Generator) makeWithdraw() bgp.UpdateEvent {
	i := g.rng.IntN(len(g.recent))
	r := g.recent[i]
	g.recent = append(g.recent[:i], g.recent[i+1:]...)
	return bgp.UpdateEvent{Kind: bgp.KindWithdraw, Prefix: r.prefix, PeerAS: r.peerAS}
}

// pickPrefix returns one of the prefixes owned by origin (origin always owns >=1).
func (g *Generator) pickPrefix(origin uint32) netip.Prefix {
	ps := g.topo.ownerPrefixes[origin]
	return ps[g.rng.IntN(len(ps))]
}

func (g *Generator) remember(ev bgp.UpdateEvent) {
	g.recent = append(g.recent, recentAnn{prefix: ev.Prefix, peerAS: ev.PeerAS})
	if len(g.recent) > recentCap {
		g.recent = g.recent[len(g.recent)-recentCap:]
	}
}
