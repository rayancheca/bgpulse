package mrt

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/rayancheca/bgpulse/backend/internal/bgp"
)

// ReplaySource is a bgp.Source that replays a pre-decoded MRT dump as a paced
// stream. It assigns monotonic IDs/Seq on emission while preserving the original
// MRT timestamps. With Loop set it restarts at the end so a small fixture keeps the
// demo alive.
type ReplaySource struct {
	events []bgp.UpdateEvent
	speed  float64 // events per second; 0 => as fast as possible
	loop   bool
}

// NewReplaySource decodes an MRT file (optionally gzip/bz2) into a replay source.
func NewReplaySource(path string, speed float64, loop bool) (*ReplaySource, error) {
	events, err := OpenFile(path)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, fmt.Errorf("mrt: %s contained no BGP UPDATE records", path)
	}
	return &ReplaySource{events: events, speed: speed, loop: loop}, nil
}

// NewReplaySourceFromBytes builds a replay source from an in-memory MRT stream
// (used for the embedded demo fixture and tests).
func NewReplaySourceFromBytes(data []byte, speed float64, loop bool) (*ReplaySource, error) {
	events, err := DecodeStream(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, fmt.Errorf("mrt: stream contained no BGP UPDATE records")
	}
	return &ReplaySource{events: events, speed: speed, loop: loop}, nil
}

// Tag identifies this source for EventID prefixes.
func (s *ReplaySource) Tag() string { return "mrt" }

// Err always returns nil: decode errors surface at construction, and per-record
// errors are skipped during decode.
func (s *ReplaySource) Err() error { return nil }

// Count returns the number of decoded events (for health/diagnostics).
func (s *ReplaySource) Count() int { return len(s.events) }

// Events replays the decoded events on the returned channel until exhausted (Loop
// off) or ctx is cancelled.
func (s *ReplaySource) Events(ctx context.Context) <-chan bgp.UpdateEvent {
	out := make(chan bgp.UpdateEvent)
	var interval time.Duration
	if s.speed > 0 {
		interval = time.Duration(float64(time.Second) / s.speed)
	}
	go func() {
		defer close(out)
		var seq uint64
		for {
			for i := range s.events {
				seq++
				ev := s.events[i]
				ev.Seq = seq
				ev.ID = bgp.EventID(fmt.Sprintf("mrt-%06d", seq))
				select {
				case out <- ev:
				case <-ctx.Done():
					return
				}
				if interval > 0 {
					select {
					case <-time.After(interval):
					case <-ctx.Done():
						return
					}
				}
			}
			if !s.loop {
				return
			}
		}
	}()
	return out
}
