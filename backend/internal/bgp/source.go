package bgp

import "context"

// Source produces a stream of normalized UpdateEvents. There is exactly one
// Source per run (the synthetic generator in demo mode, or the MRT replay parser).
//
// The Source owns the channel it returns and must close it exactly once, when ctx
// is cancelled or the underlying data is exhausted. After the channel closes, Err
// returns the terminal error (nil on a clean stop or EOF). Consumers must never
// close the channel.
type Source interface {
	// Events returns the receive-only event channel. It is called exactly once;
	// the Source spawns its own producer goroutine bound to ctx.
	Events(ctx context.Context) <-chan UpdateEvent
	// Err returns the terminal error after the channel closes (nil = clean stop).
	Err() error
	// Tag is a short identifier used as the EventID prefix ("synth", "mrt").
	Tag() string
}
