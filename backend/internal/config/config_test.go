package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Mode != ModeDemo {
		t.Errorf("mode = %q, want demo", cfg.Mode)
	}
	if cfg.ListenAddr != ":8080" {
		t.Errorf("listen = %q, want :8080", cfg.ListenAddr)
	}
	if cfg.RingCapacity != 5000 {
		t.Errorf("ring = %d, want 5000", cfg.RingCapacity)
	}
}

func TestLoadFlags(t *testing.T) {
	cfg, err := Load([]string{"-mode", "replay", "-listen", ":9000", "-seed", "7"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Mode != ModeReplay || cfg.ListenAddr != ":9000" || cfg.Seed != 7 {
		t.Errorf("flags not applied: %+v", cfg)
	}
}

func TestEnvOverridesDefault(t *testing.T) {
	t.Setenv("BGPULSE_LISTEN", ":7777")
	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ListenAddr != ":7777" {
		t.Errorf("listen = %q, want env :7777", cfg.ListenAddr)
	}
}

func TestFlagBeatsEnv(t *testing.T) {
	t.Setenv("BGPULSE_LISTEN", ":7777")
	cfg, err := Load([]string{"-listen", ":9999"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ListenAddr != ":9999" {
		t.Errorf("listen = %q, want flag :9999", cfg.ListenAddr)
	}
}

func TestValidationErrors(t *testing.T) {
	cases := map[string][]string{
		"bad mode":       {"-mode", "bogus"},
		"bad listen":     {"-listen", "not-an-address"},
		"tiny ring":      {"-ring", "10"},
		"negative speed": {"-replay-speed", "-1"},
		"bad log level":  {"-log-level", "loud"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(args); err == nil {
				t.Errorf("expected error for %v", args)
			}
		})
	}
}
