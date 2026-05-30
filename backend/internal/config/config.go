// Package config loads and validates BGPulse runtime configuration from flags and
// environment variables (flag value > env var > default), and builds the logger.
package config

import (
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
)

// Version is overridable at build time via -ldflags "-X .../config.Version=...".
var Version = "dev"

// Mode selects the data source.
type Mode string

const (
	// ModeDemo runs the deterministic synthetic generator with derived data (default).
	ModeDemo Mode = "demo"
	// ModeReplay parses a real MRT dump (or the bundled fixture) with CAIDA/VRP data.
	ModeReplay Mode = "replay"
)

// Config is the validated, immutable runtime configuration.
type Config struct {
	Mode       Mode
	ListenAddr string
	CORSOrigin string
	StaticDir  string // optional: serve a built frontend (single-binary mode)

	MRTFile   string
	ASRelFile string
	VRPFile   string
	RTRAddr   string

	Seed         uint64
	ReplaySpeed  float64
	LeakEvery    int
	HijackEvery  int
	RingCapacity int

	LogFormat string
	LogLevel  slog.Level
	Version   string
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

// Load parses flags and environment and returns a validated Config.
func Load(args []string) (Config, error) {
	fs := flag.NewFlagSet("bgpulse", flag.ContinueOnError)
	var (
		mode      = fs.String("mode", env("BGPULSE_MODE", string(ModeDemo)), "data source mode: demo | replay")
		listen    = fs.String("listen", env("BGPULSE_LISTEN", ":8080"), "HTTP listen address")
		cors      = fs.String("cors-origin", env("BGPULSE_CORS_ORIGIN", ""), "allowed CORS origin (empty = same-origin)")
		static    = fs.String("static-dir", env("BGPULSE_STATIC_DIR", ""), "directory of built frontend to serve (optional)")
		mrtFile   = fs.String("mrt-file", env("BGPULSE_MRT_FILE", ""), "MRT dump file for replay mode (empty = bundled sample)")
		asrelFile = fs.String("asrel-file", env("BGPULSE_ASREL_FILE", ""), "CAIDA AS-relationship file (empty = bundled sample)")
		vrpFile   = fs.String("vrp-file", env("BGPULSE_VRP_FILE", ""), "Routinator/rpki-client VRP JSON (empty = bundled sample)")
		rtrAddr   = fs.String("rtr-addr", env("BGPULSE_RTR_ADDR", ""), "live RTR cache host:port (overrides VRP file)")
		seed      = fs.Uint64("seed", uint64(envInt("BGPULSE_SEED", 0)), "synthetic generator seed (0 = built-in default)")
		speed     = fs.Float64("replay-speed", envFloat("BGPULSE_REPLAY_SPEED", 18), "events/sec pacing (0 = as fast as possible)")
		leak      = fs.Int("leak-every", envInt("BGPULSE_LEAK_EVERY", 12), "demo: inject a leak every N events")
		hijack    = fs.Int("hijack-every", envInt("BGPULSE_HIJACK_EVERY", 19), "demo: inject a hijack every N events")
		ring      = fs.Int("ring", envInt("BGPULSE_RING", 5000), "recent-event ring capacity")
		logFormat = fs.String("log-format", env("BGPULSE_LOG_FORMAT", "text"), "log format: text | json")
		logLevel  = fs.String("log-level", env("BGPULSE_LOG_LEVEL", "info"), "log level: debug | info | warn | error")
	)
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	cfg := Config{
		Mode: Mode(*mode), ListenAddr: *listen, CORSOrigin: *cors, StaticDir: *static,
		MRTFile: *mrtFile, ASRelFile: *asrelFile, VRPFile: *vrpFile, RTRAddr: *rtrAddr,
		Seed: *seed, ReplaySpeed: *speed, LeakEvery: *leak, HijackEvery: *hijack,
		RingCapacity: *ring, LogFormat: *logFormat, Version: Version,
	}
	if err := cfg.parseLevel(*logLevel); err != nil {
		return Config{}, err
	}
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c *Config) parseLevel(s string) error {
	switch s {
	case "debug":
		c.LogLevel = slog.LevelDebug
	case "info":
		c.LogLevel = slog.LevelInfo
	case "warn":
		c.LogLevel = slog.LevelWarn
	case "error":
		c.LogLevel = slog.LevelError
	default:
		return fmt.Errorf("config: invalid log level %q", s)
	}
	return nil
}

func (c *Config) validate() error {
	if c.Mode != ModeDemo && c.Mode != ModeReplay {
		return fmt.Errorf("config: invalid mode %q (want demo | replay)", c.Mode)
	}
	if _, _, err := net.SplitHostPort(c.ListenAddr); err != nil {
		return fmt.Errorf("config: invalid listen address %q: %w", c.ListenAddr, err)
	}
	if c.RingCapacity < 100 {
		return fmt.Errorf("config: ring capacity %d too small (min 100)", c.RingCapacity)
	}
	if c.ReplaySpeed < 0 {
		return fmt.Errorf("config: replay-speed must be >= 0")
	}
	if c.LeakEvery < 0 || c.HijackEvery < 0 {
		return fmt.Errorf("config: leak/hijack-every must be >= 0")
	}
	// An explicitly provided MRT file must exist; an empty value uses the bundle.
	if c.Mode == ModeReplay && c.MRTFile != "" {
		if _, err := os.Stat(c.MRTFile); err != nil {
			return fmt.Errorf("config: mrt-file %q not readable: %w", c.MRTFile, err)
		}
	}
	return nil
}

// NewLogger builds a slog.Logger from the config.
func NewLogger(c Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: c.LogLevel}
	if c.LogFormat == "json" {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}
