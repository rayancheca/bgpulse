# BGPulse Architecture

BGPulse is a streaming pipeline that classifies BGP updates for route leaks and
prefix hijacks and renders the result as a live AS topology. This document is the
reconciled architecture; the full specification lives in [`../CLAUDE.md`](../CLAUDE.md).

## Pipeline

```
 Source ──UpdateEvent──▶ Classifier ──ClassifiedEvent──▶ Aggregator ──▶ WebSocket Hub ──▶ browser
  │ (buf 1024)            │ (1 goroutine)      (buf 1024)  │ (single writer)   │ (drop-oldest)
  │                       │                                 │ snapReq ▲          │ marshal once
  synthetic | mrt         valley-free + RFC6811 RPKI         REST handlers ──────┘
```

- **Source** (`internal/bgp.Source`) — either the deterministic synthetic generator
  (`internal/synth`) or the MRT replay parser (`internal/mrt`). Both emit the
  identical normalized `UpdateEvent`, so everything downstream is source-agnostic.
- **Classifier** (`internal/classify`) — a single goroutine (per-prefix order is
  required by the RIB) that runs the Gao-Rexford valley-free walk (`internal/valleyfree`)
  and RFC 6811 origin validation (`internal/rpki`), then resolves the combined status
  by precedence **Hijack > Leak > Normal**.
- **Aggregator** (`internal/topology`) — the heart of the concurrency model.
- **WebSocket Hub** (`internal/wshub`) — fan-out with per-client backpressure.
- **API** (`internal/api`) — REST snapshots + the WS contract DTOs.
- **Server** (`internal/server`) — composition root: wires the stages under one
  `errgroup` with graceful shutdown.

## The single-writer aggregator

The AS graph is mutated on every event and read on every REST/snapshot request. Two
designs were considered:

1. `sync.RWMutex` around the graph — but snapshot reads must deep-copy under the lock
   (the graph holds maps and slices), stalling the high-frequency writer, and it
   invites lock-forgetting bugs as the struct grows.
2. **A single-writer actor goroutine** (chosen). One goroutine exclusively owns
   `*TopologyGraph`, the `EventRing`, and the `Stats`. It `select`s over the
   classified-event input, a snapshot-request channel (read → build an immutable value
   copy → reply), and a 1s rate ticker. No mutex anywhere.

Data races become structurally impossible — the map is never shared — and every
mutation lives in one auditable `apply()`. Verified with `go test -race`.

The `apply()` path maintains prefix counts from a per-node set (origin-change safe,
never negative), upserts **directed** edges that show their *latest* status with
cumulative leak/hijack tallies (so anomalies flash then return to normal), and skips
AS_PATH-prepending self-loops.

## Classification correctness

**Valley-free (Gao-Rexford).** Reading the AS_PATH in propagation order, the valid
shape is `(c2p)* (p2p | p2c)? (p2c)*`. A two-phase `Up → Down` machine accepts only
customer→provider links while climbing, allows at most one peer link or the first
downhill link, then accepts only provider→customer links. Any violation is a valley;
the AS at the violation is the reported offender. Sibling links are phase-transparent;
unknown and AS_SET links are never flagged (no false leaks on incomplete data).

**RFC 6811 RPKI.** *Covering* is pure prefix containment; `maxLength` is part of the
*match* test. A prefix contained by a VRP but more specific than its `maxLength`, with
no matching VRP, is **Invalid** (not `NotFound`) — the rule that catches more-specific
hijacks. VRPs are indexed in separate IPv4/IPv6 binary tries.

**RFC 8210 RTR.** The live VRP source: a Reset/Serial-Query state machine that decodes
IPv4/IPv6 Prefix PDUs, applies incremental deltas on Serial Notify, handles Cache
Reset and Error Report, and swaps the validated set into an atomic `rpki.Live` store.

## Offline determinism

Pluggability lives only at the data-source seam. In demo mode, `synth.BuildDefault`
constructs one canonical tiered topology (tier-1 peer mesh, transit, stubs) and
derives *both* the relationship store and the VRP set from it, so the classifiers are
guaranteed consistent. A seeded `math/rand/v2` generator emits a valley-free baseline
and injects reproducible leaks and hijacks. An integration test runs the synthetic
stream through the real classifier and asserts the injected anomalies are detected
with zero false positives.

## Wire contract

A typed WebSocket pushes a `snapshot` on connect, then `event` frames and periodic
`stats`. Frames are JSON with a discriminated `type`; enums are lowercase tokens,
timestamps are RFC3339, edges are directed. The Go DTOs (`internal/api/dto.go`) are
the single source of truth; the frontend infers its TypeScript types from zod schemas
that mirror them, and a golden JSON fixture is validated by both sides.

## Frontend rendering

`d3-force` owns only the layout math. A **Canvas 2D** renderer draws the entire graph
in one pass per `requestAnimationFrame` — no DOM reconciliation at hundreds of nodes,
keeping interaction latency low. React owns the chrome (status bar, rails, drawer) and
the few interactive overlays (tooltip). Two Zustand stores hold the live graph (mutated
in place so the simulation keeps node positions; a `structureVersion` bumps only on
shape change) and the bounded event ring + stats. Every inbound frame is validated
with zod at the boundary; malformed frames are dropped, never crashing the graph.
