package server

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"

	"github.com/rayancheca/bgpulse/backend/data"
	"github.com/rayancheca/bgpulse/backend/internal/api"
	"github.com/rayancheca/bgpulse/backend/internal/bgp"
	"github.com/rayancheca/bgpulse/backend/internal/classify"
	"github.com/rayancheca/bgpulse/backend/internal/config"
	"github.com/rayancheca/bgpulse/backend/internal/mrt"
	"github.com/rayancheca/bgpulse/backend/internal/relationships"
	"github.com/rayancheca/bgpulse/backend/internal/rpki"
	"github.com/rayancheca/bgpulse/backend/internal/rtr"
	"github.com/rayancheca/bgpulse/backend/internal/synth"
)

// sources bundles everything the pipeline and API need, assembled per mode.
type sources struct {
	source    bgp.Source
	rel       *relationships.RelStore
	validator classify.Validator
	rtr       *rtr.Client // non-nil only for live RTR
	health    api.HealthInfo
}

// buildSources assembles the data plane for the configured mode. Explicitly
// requested files that fail to load are fatal; absent files fall back to the
// embedded bundle so the binary always runs offline.
func buildSources(cfg config.Config, log *slog.Logger) (sources, error) {
	switch cfg.Mode {
	case config.ModeDemo:
		return demoSources(cfg), nil
	case config.ModeReplay:
		return replaySources(cfg, log)
	default:
		return sources{}, fmt.Errorf("server: unknown mode %q", cfg.Mode)
	}
}

func demoSources(cfg config.Config) sources {
	seed := cfg.Seed
	if seed == 0 {
		seed = synth.DefaultSeed
	}
	topo := synth.BuildDefault(seed)
	gen := synth.NewGenerator(topo, synth.GenConfig{
		Seed: seed, LeakEvery: cfg.LeakEvery, HijackEvery: cfg.HijackEvery,
		WithdrawProb: 0.08, EventsPerSec: cfg.ReplaySpeed,
	})
	return sources{
		source:    gen,
		rel:       topo.Rel(),
		validator: topo.VRP(),
		health: api.HealthInfo{
			Mode: string(cfg.Mode), Version: cfg.Version,
			BGPSource: "synthetic", RelSource: "synthetic", RPKISource: "synthetic",
		},
	}
}

func replaySources(cfg config.Config, log *slog.Logger) (sources, error) {
	src, bgpDesc, err := replaySource(cfg)
	if err != nil {
		return sources{}, err
	}
	rel, relDesc, err := relStore(cfg)
	if err != nil {
		return sources{}, err
	}
	validator, rtrClient, rpkiDesc, err := vrpValidator(cfg, log)
	if err != nil {
		return sources{}, err
	}
	return sources{
		source: src, rel: rel, validator: validator, rtr: rtrClient,
		health: api.HealthInfo{
			Mode: string(cfg.Mode), Version: cfg.Version,
			BGPSource: bgpDesc, RelSource: relDesc, RPKISource: rpkiDesc,
		},
	}, nil
}

func replaySource(cfg config.Config) (bgp.Source, string, error) {
	if cfg.MRTFile != "" {
		rs, err := mrt.NewReplaySource(cfg.MRTFile, cfg.ReplaySpeed, false)
		if err != nil {
			return nil, "", err
		}
		return rs, "mrt:" + cfg.MRTFile, nil
	}
	rs, err := mrt.NewReplaySourceFromBytes(data.SampleMRT(), cfg.ReplaySpeed, true)
	if err != nil {
		return nil, "", fmt.Errorf("server: bundled MRT fixture: %w", err)
	}
	return rs, "mrt:bundled-sample", nil
}

func relStore(cfg config.Config) (*relationships.RelStore, string, error) {
	if cfg.ASRelFile != "" {
		f, err := os.Open(cfg.ASRelFile)
		if err != nil {
			return nil, "", fmt.Errorf("server: asrel-file: %w", err)
		}
		defer f.Close()
		rel, err := relationships.LoadCAIDA(f)
		if err != nil {
			return nil, "", err
		}
		return rel, "caida:" + cfg.ASRelFile, nil
	}
	rel, err := relationships.LoadCAIDA(bytes.NewReader(data.SampleASRel()))
	if err != nil {
		return nil, "", fmt.Errorf("server: bundled AS-rel: %w", err)
	}
	return rel, "bundled", nil
}

func vrpValidator(cfg config.Config, log *slog.Logger) (classify.Validator, *rtr.Client, string, error) {
	if cfg.RTRAddr != "" {
		live := rpki.NewLive(nil)
		client := rtr.NewClient(cfg.RTRAddr, live, log)
		return live, client, "rtr:" + cfg.RTRAddr, nil
	}
	if cfg.VRPFile != "" {
		f, err := os.Open(cfg.VRPFile)
		if err != nil {
			return nil, nil, "", fmt.Errorf("server: vrp-file: %w", err)
		}
		defer f.Close()
		store, err := rpki.LoadVRPsJSON(f)
		if err != nil {
			return nil, nil, "", err
		}
		return store, nil, "vrp:" + cfg.VRPFile, nil
	}
	store, err := rpki.LoadVRPsJSON(bytes.NewReader(data.SampleVRPs()))
	if err != nil {
		return nil, nil, "", fmt.Errorf("server: bundled VRPs: %w", err)
	}
	return store, nil, "bundled", nil
}
