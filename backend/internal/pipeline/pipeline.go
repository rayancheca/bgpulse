// Package pipeline connects the data source to the classifier to the aggregator. A
// single classifier goroutine preserves per-prefix arrival order, which the RIB
// maintenance in the aggregator depends on.
package pipeline

import (
	"context"
	"log/slog"

	"github.com/rayancheca/bgpulse/backend/internal/bgp"
)

// Pipeline reads from a Source, classifies each event, and forwards it on out.
type Pipeline struct {
	source     bgp.Source
	classifier bgp.Classifier
	out        chan bgp.ClassifiedEvent
	log        *slog.Logger
}

// New builds a pipeline. The pipeline owns out and closes it when it stops, which
// signals the aggregator to drain and return.
func New(source bgp.Source, classifier bgp.Classifier, out chan bgp.ClassifiedEvent, log *slog.Logger) *Pipeline {
	return &Pipeline{source: source, classifier: classifier, out: out, log: log}
}

// Run classifies events until the source is exhausted or ctx is cancelled.
func (p *Pipeline) Run(ctx context.Context) error {
	defer close(p.out)
	src := p.source.Events(ctx)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev, ok := <-src:
			if !ok {
				if err := p.source.Err(); err != nil {
					return err
				}
				p.log.Info("source exhausted", "source", p.source.Tag())
				return nil
			}
			classified := p.classifier.Classify(ev)
			select {
			case p.out <- classified:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}
