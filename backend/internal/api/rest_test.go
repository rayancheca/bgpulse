package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/rayancheca/bgpulse/backend/internal/bgp"
	"github.com/rayancheca/bgpulse/backend/internal/topology"
)

// fakeStore returns canned views for handler testing.
type fakeStore struct{}

func (fakeStore) Topology() topology.SnapshotView {
	return topology.SnapshotView{
		Nodes:       []topology.NodeView{{ASN: 100, Name: "Test", PrefixCount: 2}},
		Edges:       []topology.EdgeView{{From: 100, To: 200, Status: bgp.VFLeak, Rel: bgp.RelProvider, Count: 3}},
		GeneratedAt: time.Unix(1_700_000_000, 0),
	}
}
func (fakeStore) Events(limit int) []bgp.ClassifiedEvent {
	return []bgp.ClassifiedEvent{sampleEvent()}
}
func (fakeStore) Stats() topology.StatsView {
	return topology.StatsView{
		TotalEvents: 10, Leaks: 2, Hijacks: 1, NodeCount: 5, EdgeCount: 7, EventsPerSec: 12.5,
		TopOrigins: []topology.OriginStatView{{ASN: 174, RPKI: bgp.RPKIValid, Throughput: []int{1, 2, 3}}},
	}
}
func (fakeStore) ASNDetail(asn uint32) topology.ASNDetailView {
	if asn != 100 {
		return topology.ASNDetailView{Found: false}
	}
	return topology.ASNDetailView{Found: true, Node: topology.NodeView{ASN: 100}, Prefixes: []string{"10.0.0.0/24"}}
}
func (fakeStore) EdgeDetail(from, to uint32, limit int) topology.EdgeDetailView {
	if from != 100 || to != 200 {
		return topology.EdgeDetailView{Found: false}
	}
	return topology.EdgeDetailView{Found: true, Edge: topology.EdgeView{From: 100, To: 200}}
}
func (fakeStore) Snapshot() topology.FullSnapshot { return topology.FullSnapshot{} }

func sampleEvent() bgp.ClassifiedEvent {
	return bgp.ClassifiedEvent{
		Event: bgp.UpdateEvent{
			ID: "synth-000042", Seq: 42, Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			Kind: bgp.KindAnnounce, Prefix: netip.MustParsePrefix("10.0.0.0/24"),
			PeerAS: 65001, ASPath: []uint32{65001, 3356, 174}, NextHop: netip.MustParseAddr("192.0.2.1"),
			Communities: []bgp.Community{{ASN: 2604, Value: 100}}, OriginAS: 174,
		},
		VFStatus: bgp.VFValid, RPKIStatus: bgp.RPKIValid,
		Hops: []bgp.PathHop{
			{From: 65001, To: 3356, Rel: bgp.RelProvider},
			{From: 3356, To: 174, Rel: bgp.RelProvider},
		},
	}
}

func testServer() http.Handler {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewServer(fakeStore{}, log, HealthInfo{Mode: "demo", Version: "test"}, "", time.Now()).Routes(nil)
}

func TestRESTStatusCodes(t *testing.T) {
	srv := testServer()
	cases := []struct {
		path string
		want int
	}{
		{"/api/health", 200},
		{"/api/topology", 200},
		{"/api/stats", 200},
		{"/api/events", 200},
		{"/api/events?limit=5", 200},
		{"/api/events?limit=abc", 400},
		{"/api/asn/100", 200},
		{"/api/asn/999", 404},
		{"/api/asn/notanumber", 400},
		{"/api/edge/100/200", 200},
		{"/api/edge/100/999", 404},
		{"/api/edge/x/200", 400},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
			if rec.Code != tc.want {
				t.Errorf("GET %s = %d, want %d (body: %s)", tc.path, rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

func TestTopologyEnvelope(t *testing.T) {
	srv := testServer()
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/topology", nil))

	var env Envelope[TopologyDTO]
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !env.OK || env.Data == nil {
		t.Fatalf("envelope ok=%v data=%v", env.OK, env.Data)
	}
	if len(env.Data.Nodes) != 1 || env.Data.Nodes[0].ASN != 100 {
		t.Errorf("topology nodes = %+v", env.Data.Nodes)
	}
	if env.Data.Edges[0].Status != "leak" {
		t.Errorf("edge status = %q, want leak", env.Data.Edges[0].Status)
	}
}

// TestClassifiedDTOGolden pins the exact wire JSON so the frontend zod schema can
// validate against the same shape. Any field rename/retag breaks this test.
func TestClassifiedDTOGolden(t *testing.T) {
	dto := classifiedToDTO(sampleEvent())
	got, err := json.Marshal(dto)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `{"id":"synth-000042","seq":42,"timestamp":"2024-01-01T00:00:00Z","kind":"announce","prefix":"10.0.0.0/24","peerAs":65001,"asPath":[65001,3356,174],"nextHop":"192.0.2.1","communities":[{"asn":2604,"value":100}],"originAs":174,"vfStatus":"valid","rpkiStatus":"valid","hops":[{"from":65001,"to":3356,"rel":"provider","isOffender":false},{"from":3356,"to":174,"rel":"provider","isOffender":false}],"offenderAs":0,"reason":""}`
	if string(got) != want {
		t.Errorf("wire contract drift:\n got: %s\nwant: %s", got, want)
	}
}
