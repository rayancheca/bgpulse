package synth

import (
	"fmt"
	"testing"

	"github.com/rayancheca/bgpulse/backend/internal/bgp"
	"github.com/rayancheca/bgpulse/backend/internal/classify"
)

func fingerprint(ev bgp.UpdateEvent) string {
	return fmt.Sprintf("%s|%s|%s|%v|%d", ev.ID, ev.Kind, ev.Prefix, ev.ASPath, ev.OriginAS)
}

func TestGeneratorDeterministic(t *testing.T) {
	cfg := DefaultGenConfig()
	cfg.EventsPerSec = 0 // no pacing in tests
	g1 := NewGenerator(BuildDefault(cfg.Seed), cfg)
	g2 := NewGenerator(BuildDefault(cfg.Seed), cfg)

	for i := 0; i < 300; i++ {
		a, b := fingerprint(g1.Next()), fingerprint(g2.Next())
		if a != b {
			t.Fatalf("event %d diverged:\n a=%s\n b=%s", i, a, b)
		}
	}
}

// TestSyntheticStreamLightsUpClassifiers is the integration test the audit demanded:
// run the synthetic stream through the REAL classifier built from the SAME topology,
// and prove both anomaly classes are produced and detected — with no false positives
// on the baseline.
func TestSyntheticStreamLightsUpClassifiers(t *testing.T) {
	cfg := DefaultGenConfig()
	cfg.EventsPerSec = 0
	topo := BuildDefault(cfg.Seed)
	gen := NewGenerator(topo, cfg)
	cls := classify.New(topo.Rel(), topo.VRP())

	const n = 600
	var leaks, hijacks int
	for i := 0; i < n; i++ {
		ev := gen.Next()
		ce := cls.Classify(ev)

		switch ce.VFStatus {
		case bgp.VFLeak:
			leaks++
			// Every leak comes from the AS2001 provider-to-provider scenario.
			if ce.OffenderAS != 2001 {
				t.Errorf("leak offender = %d, want 2001 (seq %d, path %v)", ce.OffenderAS, ev.Seq, ev.ASPath)
			}
		case bgp.VFHijack:
			hijacks++
			if ce.RPKIStatus != bgp.RPKIInvalid {
				t.Errorf("hijack must be RPKI-Invalid, got %v (seq %d)", ce.RPKIStatus, ev.Seq)
			}
			if ce.OffenderAS < stubBase {
				t.Errorf("hijack offender = %d, want a stub (>=%d)", ce.OffenderAS, stubBase)
			}
		}

		// No false positives: an announce that is NOT at a scheduled anomaly slot must
		// never be classified as a leak or hijack.
		scheduled := ev.Seq%uint64(cfg.LeakEvery) == 0 || ev.Seq%uint64(cfg.HijackEvery) == 0
		if ev.Kind == bgp.KindAnnounce && !scheduled {
			if ce.VFStatus == bgp.VFLeak || ce.VFStatus == bgp.VFHijack {
				t.Errorf("false positive at seq %d: baseline announce classified %v (path %v, prefix %s)",
					ev.Seq, ce.VFStatus, ev.ASPath, ev.Prefix)
			}
		}
	}

	if leaks == 0 {
		t.Error("expected at least one detected leak over the stream")
	}
	if hijacks == 0 {
		t.Error("expected at least one detected hijack over the stream")
	}
	t.Logf("over %d events: %d leaks, %d hijacks detected", n, leaks, hijacks)
}

func TestClassificationDeterministic(t *testing.T) {
	cfg := DefaultGenConfig()
	cfg.EventsPerSec = 0
	run := func() []bgp.VFStatus {
		topo := BuildDefault(cfg.Seed)
		gen := NewGenerator(topo, cfg)
		cls := classify.New(topo.Rel(), topo.VRP())
		out := make([]bgp.VFStatus, 0, 200)
		for i := 0; i < 200; i++ {
			out = append(out, cls.Classify(gen.Next()).VFStatus)
		}
		return out
	}
	a, b := run(), run()
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("classification diverged at %d: %v vs %v", i, a[i], b[i])
		}
	}
}
