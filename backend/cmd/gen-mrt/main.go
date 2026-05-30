// Command gen-mrt regenerates the bundled demo MRT fixture (data/updates.sample.mrt)
// that replay mode embeds. Run from the backend/ directory: go run ./cmd/gen-mrt
package main

import (
	"log"
	"os"

	"github.com/rayancheca/bgpulse/backend/internal/mrt"
)

func main() {
	data, err := mrt.BuildSampleMRT()
	if err != nil {
		log.Fatalf("build sample MRT: %v", err)
	}
	const out = "data/updates.sample.mrt"
	if err := os.WriteFile(out, data, 0o644); err != nil {
		log.Fatalf("write %s: %v", out, err)
	}
	log.Printf("wrote %s (%d bytes)", out, len(data))
}
