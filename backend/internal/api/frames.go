package api

import (
	"encoding/json"

	"github.com/rayancheca/bgpulse/backend/internal/bgp"
	"github.com/rayancheca/bgpulse/backend/internal/topology"
)

// EventFrame marshals a classified event as a WebSocket "event" frame.
func EventFrame(seq uint64, ev bgp.ClassifiedEvent) ([]byte, error) {
	dto := classifiedToDTO(ev)
	return json.Marshal(WSMessage{Type: WSEvent, Seq: seq, Event: &dto})
}

// StatsFrame marshals a stats view as a WebSocket "stats" frame.
func StatsFrame(seq uint64, s topology.StatsView) ([]byte, error) {
	dto := statsViewToDTO(s)
	return json.Marshal(WSMessage{Type: WSStats, Seq: seq, Stats: &dto})
}

// SnapshotFrame marshals a full snapshot as the on-connect WebSocket "snapshot" frame.
func SnapshotFrame(s topology.FullSnapshot) ([]byte, error) {
	dto := snapshotToDTO(s)
	return json.Marshal(WSMessage{Type: WSSnapshot, Snapshot: &dto})
}
