package config

import "time"

// Pipeline and server tuning constants kept in one place.
const (
	// ClassifiedBufSize buffers events between the classifier and the aggregator.
	ClassifiedBufSize = 1024
	// BroadcastBufSize buffers the aggregator->hub broadcast channel.
	BroadcastBufSize = 256
	// ShutdownTimeout bounds graceful HTTP shutdown.
	ShutdownTimeout = 5 * time.Second
)
