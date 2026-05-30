package topology

import "github.com/rayancheca/bgpulse/backend/internal/bgp"

// EventRing is a fixed-capacity ring of the most recent ClassifiedEvents. It is
// owned by the Aggregator goroutine; snapshot reads copy out.
type EventRing struct {
	buf  []bgp.ClassifiedEvent
	head int // index of the next write
	size int
	cap  int
}

func newEventRing(capacity int) *EventRing {
	if capacity < 1 {
		capacity = 1
	}
	return &EventRing{buf: make([]bgp.ClassifiedEvent, capacity), cap: capacity}
}

// push appends, overwriting the oldest when full.
func (r *EventRing) push(ev bgp.ClassifiedEvent) {
	r.buf[r.head] = ev
	r.head = (r.head + 1) % r.cap
	if r.size < r.cap {
		r.size++
	}
}

// recent returns up to limit most-recent events, newest first, as a fresh slice.
func (r *EventRing) recent(limit int) []bgp.ClassifiedEvent {
	if limit <= 0 || limit > r.size {
		limit = r.size
	}
	out := make([]bgp.ClassifiedEvent, 0, limit)
	for i := 0; i < limit; i++ {
		idx := (r.head - 1 - i + 2*r.cap) % r.cap
		out = append(out, r.buf[idx])
	}
	return out
}

// byEdge returns up to limit recent events whose AS_PATH traverses the directed
// adjacency from->to, newest first.
func (r *EventRing) byEdge(from, to uint32, limit int) []bgp.ClassifiedEvent {
	out := make([]bgp.ClassifiedEvent, 0, limit)
	for i := 0; i < r.size && len(out) < limit; i++ {
		idx := (r.head - 1 - i + 2*r.cap) % r.cap
		ev := r.buf[idx]
		if pathHasEdge(ev.Event.ASPath, from, to) {
			out = append(out, ev)
		}
	}
	return out
}

// pathHasEdge reports whether path contains the adjacent ordered pair from->to.
func pathHasEdge(path []uint32, from, to uint32) bool {
	for i := 0; i+1 < len(path); i++ {
		if path[i] == from && path[i+1] == to {
			return true
		}
	}
	return false
}
