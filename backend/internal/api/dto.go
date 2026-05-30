// Package api defines the BGPulse HTTP/REST + WebSocket surface and the JSON DTOs
// that are the single source of truth for the wire contract. Enums serialize as
// lowercase string tokens, timestamps as RFC3339 strings, communities as {asn,value}
// objects, and edges are directed. The frontend's TypeScript types and zod schemas
// mirror these exactly.
package api

// Envelope is the uniform wrapper for every REST response.
type Envelope[T any] struct {
	OK    bool   `json:"ok"`
	Data  *T     `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

// CommunityDTO is a standard BGP community.
type CommunityDTO struct {
	ASN   uint16 `json:"asn"`
	Value uint16 `json:"value"`
}

// PathHopDTO annotates one wire-order adjacency on the AS_PATH.
type PathHopDTO struct {
	From       uint32 `json:"from"`
	To         uint32 `json:"to"`
	Rel        string `json:"rel"`
	IsOffender bool   `json:"isOffender"`
}

// ClassifiedDTO is the wire form of a classified BGP event.
type ClassifiedDTO struct {
	ID          string         `json:"id"`
	Seq         uint64         `json:"seq"`
	Timestamp   string         `json:"timestamp"`
	Kind        string         `json:"kind"`
	Prefix      string         `json:"prefix"`
	PeerAs      uint32         `json:"peerAs"`
	ASPath      []uint32       `json:"asPath"`
	NextHop     string         `json:"nextHop"`
	Communities []CommunityDTO `json:"communities"`
	OriginAs    uint32         `json:"originAs"`
	VfStatus    string         `json:"vfStatus"`
	RpkiStatus  string         `json:"rpkiStatus"`
	Hops        []PathHopDTO   `json:"hops"`
	OffenderAs  uint32         `json:"offenderAs"`
	Reason      string         `json:"reason"`
}

// RPKICountsDTO tallies origin-validation verdicts for an AS.
type RPKICountsDTO struct {
	Valid    int `json:"valid"`
	Invalid  int `json:"invalid"`
	NotFound int `json:"notfound"`
}

// NodeDTO is the wire form of an AS node.
type NodeDTO struct {
	ASN         uint32        `json:"asn"`
	Name        string        `json:"name"`
	PrefixCount int           `json:"prefixCount"`
	RPKI        RPKICountsDTO `json:"rpki"`
	FirstSeen   string        `json:"firstSeen"`
	LastSeen    string        `json:"lastSeen"`
}

// EdgeDTO is the wire form of a directed edge.
type EdgeDTO struct {
	From        uint32 `json:"from"`
	To          uint32 `json:"to"`
	Status      string `json:"status"`
	Rel         string `json:"rel"`
	Count       int    `json:"count"`
	LeakCount   int    `json:"leakCount"`
	HijackCount int    `json:"hijackCount"`
	LastEvent   string `json:"lastEvent"`
	LastEventId string `json:"lastEventId"`
}

// TopologyDTO is a full graph snapshot.
type TopologyDTO struct {
	Nodes     []NodeDTO `json:"nodes"`
	Edges     []EdgeDTO `json:"edges"`
	NodeCount int       `json:"nodeCount"`
	EdgeCount int       `json:"edgeCount"`
	Generated string    `json:"generated"`
}

// OriginStatDTO feeds one RPKI sidebar card.
type OriginStatDTO struct {
	ASN         uint32 `json:"asn"`
	Name        string `json:"name"`
	PrefixCount int    `json:"prefixCount"`
	RpkiStatus  string `json:"rpkiStatus"`
	Valid       int    `json:"valid"`
	Invalid     int    `json:"invalid"`
	NotFound    int    `json:"notfound"`
	Throughput  []int  `json:"throughput"`
}

// StatsDTO holds the running counters plus the top origins for the sidebar.
type StatsDTO struct {
	TotalEvents  int64           `json:"totalEvents"`
	Announces    int64           `json:"announces"`
	Withdraws    int64           `json:"withdraws"`
	Leaks        int64           `json:"leaks"`
	Hijacks      int64           `json:"hijacks"`
	RpkiValid    int64           `json:"rpkiValid"`
	RpkiInvalid  int64           `json:"rpkiInvalid"`
	RpkiNotFound int64           `json:"rpkiNotFound"`
	NodeCount    int             `json:"nodeCount"`
	EdgeCount    int             `json:"edgeCount"`
	EventsPerSec float64         `json:"eventsPerSec"`
	TopOrigins   []OriginStatDTO `json:"topOrigins"`
}

// EventsDTO is the recent-events list response.
type EventsDTO struct {
	Events []ClassifiedDTO `json:"events"`
	Count  int             `json:"count"`
	Limit  int             `json:"limit"`
}

// NeighborDTO describes one adjacency of an AS.
type NeighborDTO struct {
	ASN       uint32 `json:"asn"`
	Direction string `json:"direction"`
	Rel       string `json:"rel"`
	Status    string `json:"status"`
}

// ASNDetailDTO is the per-AS detail response.
type ASNDetailDTO struct {
	Node      NodeDTO       `json:"node"`
	Neighbors []NeighborDTO `json:"neighbors"`
	Prefixes  []string      `json:"prefixes"`
	Sparkline []int         `json:"sparkline"`
}

// EdgeDetailDTO is the per-edge drill-down response.
type EdgeDetailDTO struct {
	Edge   EdgeDTO         `json:"edge"`
	Events []ClassifiedDTO `json:"events"`
}

// SnapshotDTO is the full bundle sent over WebSocket on connect.
type SnapshotDTO struct {
	Topology TopologyDTO     `json:"topology"`
	Events   []ClassifiedDTO `json:"events"`
	Stats    StatsDTO        `json:"stats"`
}

// SourceHealthDTO describes the active data sources.
type SourceHealthDTO struct {
	BGP           string `json:"bgp"`
	Relationships string `json:"relationships"`
	RPKI          string `json:"rpki"`
	LiveFellBack  bool   `json:"liveFellBack"`
}

// HealthDTO is the liveness + mode response.
type HealthDTO struct {
	OK        bool            `json:"ok"`
	Mode      string          `json:"mode"`
	Version   string          `json:"version"`
	UptimeSec int64           `json:"uptimeSec"`
	Sources   SourceHealthDTO `json:"sources"`
}

// WS frame type discriminators.
const (
	WSSnapshot = "snapshot"
	WSEvent    = "event"
	WSStats    = "stats"
)

// WSMessage is the envelope for every server->client WebSocket frame. Exactly one
// payload field is set, matching Type.
type WSMessage struct {
	Type     string         `json:"type"`
	Seq      uint64         `json:"seq,omitempty"`
	Event    *ClassifiedDTO `json:"event,omitempty"`
	Stats    *StatsDTO      `json:"stats,omitempty"`
	Snapshot *SnapshotDTO   `json:"snapshot,omitempty"`
}
