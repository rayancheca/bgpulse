package synth

import (
	"testing"

	"github.com/rayancheca/bgpulse/backend/internal/bgp"
)

func TestBuildDefaultCounts(t *testing.T) {
	topo := BuildDefault(DefaultSeed)
	if len(topo.Tier1) != tier1Count || len(topo.Transit) != transitCount || len(topo.Stubs) != stubCount {
		t.Fatalf("counts: tier1=%d transit=%d stubs=%d", len(topo.Tier1), len(topo.Transit), len(topo.Stubs))
	}
	if topo.Rel().Size() == 0 || topo.VRP().Size() == 0 {
		t.Fatalf("empty stores: rel=%d vrp=%d", topo.Rel().Size(), topo.VRP().Size())
	}
	if len(topo.originables) == 0 {
		t.Fatal("no originable ASes")
	}
}

func TestTier1MeshAndForcedMultihome(t *testing.T) {
	topo := BuildDefault(DefaultSeed)
	rel := topo.Rel()
	// Tier-1s are a full peering mesh.
	for i := 0; i < len(topo.Tier1); i++ {
		for j := i + 1; j < len(topo.Tier1); j++ {
			if got := rel.Lookup(topo.Tier1[i], topo.Tier1[j]); got != bgp.RelPeer {
				t.Errorf("tier1 %d-%d = %v, want peer", topo.Tier1[i], topo.Tier1[j], got)
			}
		}
	}
	// AS2001 is forced to be a customer of both AS1001 and AS1002 (leak scenario).
	if got := rel.Lookup(2001, 1001); got != bgp.RelProvider {
		t.Errorf("Lookup(2001,1001) = %v, want provider", got)
	}
	if got := rel.Lookup(2001, 1002); got != bgp.RelProvider {
		t.Errorf("Lookup(2001,1002) = %v, want provider", got)
	}
}

func TestBuildDefaultDeterministic(t *testing.T) {
	a := BuildDefault(DefaultSeed)
	b := BuildDefault(DefaultSeed)
	if a.Rel().Size() != b.Rel().Size() || a.VRP().Size() != b.VRP().Size() {
		t.Fatalf("non-deterministic store sizes: rel %d/%d vrp %d/%d",
			a.Rel().Size(), b.Rel().Size(), a.VRP().Size(), b.VRP().Size())
	}
	if len(a.originables) != len(b.originables) {
		t.Fatalf("non-deterministic originables: %d/%d", len(a.originables), len(b.originables))
	}
}

func TestEveryOriginableOwnsPrefix(t *testing.T) {
	topo := BuildDefault(DefaultSeed)
	for _, asn := range topo.originables {
		if len(topo.ownerPrefixes[asn]) == 0 {
			t.Errorf("AS%d is originable but owns no prefix", asn)
		}
	}
}
