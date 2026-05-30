package rpki

import (
	"net/netip"
	"sync/atomic"
)

// Live is a concurrency-safe, swappable VRP validator. Reads on the hot path always
// see a consistent immutable VRPStore; the RTR client replaces the whole store
// atomically on each sync. It satisfies the classify.Validator interface, so demo
// (static) and live (RTR) modes are interchangeable behind it.
type Live struct {
	ptr atomic.Pointer[VRPStore]
}

// NewLive wraps an initial store (an empty store is used if nil).
func NewLive(initial *VRPStore) *Live {
	if initial == nil {
		initial = NewBuilder().Build()
	}
	l := &Live{}
	l.ptr.Store(initial)
	return l
}

// Validate validates against the current store.
func (l *Live) Validate(prefix netip.Prefix, origin uint32) Result {
	return l.ptr.Load().Validate(prefix, origin)
}

// Replace atomically swaps in a new store. A nil store is ignored.
func (l *Live) Replace(s *VRPStore) {
	if s != nil {
		l.ptr.Store(s)
	}
}

// Size reports the size of the current store.
func (l *Live) Size() int { return l.ptr.Load().Size() }
