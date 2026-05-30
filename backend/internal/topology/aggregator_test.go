package topology

import (
	"context"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/rayancheca/bgpulse/backend/internal/bgp"
	"github.com/rayancheca/bgpulse/backend/internal/classify"
	"github.com/rayancheca/bgpulse/backend/internal/synth"
)

func annEvent(id, prefix string, path []uint32, vf bgp.VFStatus, rpki bgp.RPKIStatus) bgp.ClassifiedEvent {
	var origin uint32
	if n := len(path); n > 0 {
		origin = path[n-1]
	}
	return bgp.ClassifiedEvent{
		Event: bgp.UpdateEvent{
			ID: bgp.EventID(id), Kind: bgp.KindAnnounce, Prefix: netip.MustParsePrefix(prefix),
			ASPath: path, OriginAS: origin, Timestamp: time.Unix(1_700_000_000, 0),
		},
		VFStatus: vf, RPKIStatus: rpki,
	}
}

func wdEvent(prefix string) bgp.ClassifiedEvent {
	return bgp.ClassifiedEvent{Event: bgp.UpdateEvent{Kind: bgp.KindWithdraw, Prefix: netip.MustParsePrefix(prefix)}}
}

func newTestAgg() *Aggregator { return NewAggregator(nil, nil, 100, nil) }

func TestApplyCreatesNodesAndEdges(t *testing.T) {
	a := newTestAgg()
	a.apply(annEvent("e1", "10.0.0.0/24", []uint32{100, 200, 300}, bgp.VFValid, bgp.RPKIValid))

	if len(a.graph.Nodes) != 3 {
		t.Fatalf("nodes = %d, want 3", len(a.graph.Nodes))
	}
	if n := a.graph.Nodes[300]; n.PrefixCount() != 1 || n.RPKI.Valid != 1 {
		t.Errorf("origin node 300: prefixCount=%d valid=%d, want 1/1", n.PrefixCount(), n.RPKI.Valid)
	}
	for _, k := range []EdgeKey{{100, 200}, {200, 300}} {
		if _, ok := a.graph.Edges[k]; !ok {
			t.Errorf("missing edge %v", k)
		}
	}
}

func TestApplyWithdrawReleasesPrefix(t *testing.T) {
	a := newTestAgg()
	a.apply(annEvent("e1", "10.0.0.0/24", []uint32{100, 300}, bgp.VFValid, bgp.RPKIValid))
	if a.graph.Nodes[300].PrefixCount() != 1 {
		t.Fatalf("prefixCount after announce = %d, want 1", a.graph.Nodes[300].PrefixCount())
	}
	a.apply(wdEvent("10.0.0.0/24"))
	if a.graph.Nodes[300].PrefixCount() != 0 {
		t.Errorf("prefixCount after withdraw = %d, want 0", a.graph.Nodes[300].PrefixCount())
	}
}

func TestApplyOriginChange(t *testing.T) {
	a := newTestAgg()
	a.apply(annEvent("e1", "10.0.0.0/24", []uint32{100, 300}, bgp.VFValid, bgp.RPKIValid))
	a.apply(annEvent("e2", "10.0.0.0/24", []uint32{100, 400}, bgp.VFHijack, bgp.RPKIInvalid))
	if got := a.graph.Nodes[300].PrefixCount(); got != 0 {
		t.Errorf("old origin 300 prefixCount = %d, want 0", got)
	}
	if got := a.graph.Nodes[400].PrefixCount(); got != 1 {
		t.Errorf("new origin 400 prefixCount = %d, want 1", got)
	}
}

func TestApplySkipsSelfLoop(t *testing.T) {
	a := newTestAgg()
	a.apply(annEvent("e1", "10.0.0.0/24", []uint32{100, 100, 200}, bgp.VFValid, bgp.RPKIValid))
	if _, ok := a.graph.Edges[EdgeKey{100, 100}]; ok {
		t.Error("self-loop edge should be skipped")
	}
	if _, ok := a.graph.Edges[EdgeKey{100, 200}]; !ok {
		t.Error("missing edge 100->200")
	}
}

func TestEdgeShowsLatestStatusButCumulativeCounts(t *testing.T) {
	a := newTestAgg()
	a.apply(annEvent("e1", "10.0.0.0/24", []uint32{100, 200}, bgp.VFLeak, bgp.RPKINotFound))
	a.apply(annEvent("e2", "10.0.0.0/24", []uint32{100, 200}, bgp.VFValid, bgp.RPKIValid))
	ed := a.graph.Edges[EdgeKey{100, 200}]
	if ed.Status != bgp.VFValid {
		t.Errorf("edge status = %v, want valid (latest)", ed.Status)
	}
	if ed.Count != 2 || ed.LeakCount != 1 {
		t.Errorf("edge count/leak = %d/%d, want 2/1", ed.Count, ed.LeakCount)
	}
}

func TestEdgeDetailReturnsTraversingEvents(t *testing.T) {
	a := newTestAgg()
	a.apply(annEvent("e1", "10.0.0.0/24", []uint32{100, 200, 300}, bgp.VFValid, bgp.RPKIValid))
	a.apply(annEvent("e2", "20.0.0.0/24", []uint32{400, 500}, bgp.VFValid, bgp.RPKIValid))
	d := a.buildEdgeDetail(100, 200, 10)
	if !d.Found || len(d.Events) != 1 || d.Events[0].Event.ID != "e1" {
		t.Errorf("edge detail (100,200) = %+v, want 1 event e1", d)
	}
}

// TestAggregatorConcurrent runs the real actor with the synthetic stream and the
// real classifier while concurrently hammering snapshot reads, under -race.
func TestAggregatorConcurrent(t *testing.T) {
	topo := synth.BuildDefault(synth.DefaultSeed)
	cls := classify.New(topo.Rel(), topo.VRP())
	cfg := synth.DefaultGenConfig()
	cfg.EventsPerSec = 0
	gen := synth.NewGenerator(topo, cfg)

	in := make(chan bgp.ClassifiedEvent, 64)
	out := make(chan bgp.ClassifiedEvent, 256)
	go func() {
		for range out { // drain broadcasts
		}
	}()
	a := NewAggregator(in, out, 500, topo.Rel())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go a.Run(ctx)

	stop := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					a.Topology()
					a.Stats()
					a.Events(10)
					a.ASNDetail(2001)
					a.EdgeDetail(1001, 2001, 5)
				}
			}
		}()
	}

	const n = 400
	for i := 0; i < n; i++ {
		in <- cls.Classify(gen.Next())
	}

	deadline := time.Now().Add(3 * time.Second)
	for a.Stats().TotalEvents < int64(n) && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	close(stop)
	wg.Wait()

	st := a.Stats()
	if st.TotalEvents != int64(n) {
		t.Errorf("TotalEvents = %d, want %d", st.TotalEvents, n)
	}
	if st.Leaks == 0 || st.Hijacks == 0 {
		t.Errorf("expected leaks and hijacks in graph, got %d/%d", st.Leaks, st.Hijacks)
	}
	if st.NodeCount == 0 || st.EdgeCount == 0 {
		t.Error("graph should not be empty")
	}
	if len(st.TopOrigins) == 0 {
		t.Error("expected top origins for the sidebar")
	}
}
