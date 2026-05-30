package bgp

// Classifier enriches a single UpdateEvent into a ClassifiedEvent. Implementations
// must be pure with respect to the event and safe to call repeatedly: all state
// (relationship graph, VRP store) is injected at construction and is read-only
// while streaming.
type Classifier interface {
	Classify(ev UpdateEvent) ClassifiedEvent
}
