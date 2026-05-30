# Agent Instructions — Read Before Every Action

Read in this order, every session:

1. `~/daily-builder/prompts/rules/session_protocol.md`
2. `~/daily-builder/prompts/rules/quality_bar.md`
3. `~/daily-builder/prompts/rules/code_rules.md`
4. `state.md` in this directory
5. This file

## How to work
- Work SLOWLY and CAREFULLY. One implementation step at a time — never combine, never skip.
- After every step: run it (`go test ./...`, build, or browser), see the output, fix errors, THEN commit and update `state.md`.
- A step is NOT done until it runs without errors and does what it says. No stubs, no TODOs, no placeholders.
- Types everywhere (Go: documented exported identifiers; TS: strict mode). Every error handled.
- The UI must look genuinely polished — see the Visual Direction section. Default browser styling = not done.
- Never leave broken code committed. Never use `--no-verify` / `--force`.

---

# Project: BGPulse

**Tagline:** Live BGP route-leak and prefix-hijack detector with AS-path topology visualization.

**Domain:** Network Engineering and Distributed Systems.

**Tech stack:** Go 1.26 (backend), TypeScript + React 19 + D3.js + Vite (frontend), Docker Compose.

**Problem:** BGP route leaks and prefix hijacks — responsible for the Facebook 2021 outage and repeated AWS/Cloudflare incidents — propagate for minutes before operators notice, and existing tools give no real-time visual signal of valley-free violations or RPKI mismatches. BGPulse ingests BGP update streams, runs the Gao-Rexford valley-free classifier and RFC 6811 RPKI origin validation in real time, and surfaces every anomaly the moment it propagates through the routing table.

**Why it stands out:** It implements the Gao-Rexford valley-free routing model, RFC 6811 RPKI Route Origin Validation, and an RFC 8210 RPKI-to-Router client — the core algorithms behind every major internet routing-security paper — against real MRT/BGP wire data, turned into an interactive force-directed AS topology that most engineers have only read about.

**Target user:** Network/routing-security engineers, NOC operators during an incident, BGP researchers.

---

# 3a–3c. Architecture, data flow, tech stack

## Pipeline (single binary, Go)

```
Source ──UpdateEvent (buf 1024)──▶ Classifier (1 goroutine) ──ClassifiedEvent (buf 1024)──▶
  Aggregator (single-writer actor, owns the graph) ──BroadcastMsg (buf 256)──▶ WS Hub ──per-client (buf 64, drop-oldest)──▶ browsers
                              ▲ snapReq/reply channel
                   REST handlers (read consistent snapshots)
```

- **Source** (`bgp.Source` interface): either the deterministic **synthetic generator** (demo, default) or the **MRT file replay** parser. Both emit the identical normalized `UpdateEvent`, so everything downstream is source-agnostic.
- **Classifier** (`bgp.Classifier`): pure & stateless; injects the immutable `RelStore` (AS relationships) + `VRPStore` (RPKI). Runs as **ONE goroutine** so per-prefix order is preserved (RIB counts must observe arrival order). For each event: runs the valley-free walk + RFC 6811 validation, builds an immutable `ClassifiedEvent`.
- **Aggregator** (`topology.Aggregator`): **single-writer goroutine** exclusively owning `*TopologyGraph`, `*EventRing`, `Stats`. No mutex anywhere — reads come in as `snapReq` over a channel, the writer replies with an immutable value copy. This makes data races structurally impossible (`go test -race` clean).
- **WS Hub** (`wshub.Hub`): single goroutine, fans `BroadcastMsg` to clients; marshals once, sends many; per-client **drop-oldest** then disconnect-after-threshold so a slow browser never stalls the system. Every client gets a full `snapshot` on connect, so deltas are advisory and reconnect auto-recovers.

## Key design decisions (and why)
1. **Single-writer actor for the graph** over RWMutex+deep-copy — eliminates races structurally, keeps all mutation in one auditable `apply()`.
2. **Value-passing immutable events** — `ClassifiedEvent` embeds `UpdateEvent` by value; no stage can mutate another's data.
3. **Pluggability lives ONLY at the data-source seam** (`Source`, `RelStore`, `VRPStore`). The algorithms are identical offline and live. Bundled fixtures are embedded with `//go:embed` so the binary runs fully offline with zero external files.
4. **Deterministic synthetic generator** (`math/rand/v2` PCG, fixed `uint64` seed, virtual clock, scheduled leak/hijack injection) — gives reproducible golden-stream tests and a demo where the classifiers reliably light up.
5. **gobgp v4 for the byte-level MRT/BGP wire decode** (`github.com/osrg/gobgp/v4/pkg/packet/{mrt,bgp}`), hand-rolled everything above it. The wire format (2-byte/4-byte ASN, AS_TRANS 23456, MP_REACH, TABLE_DUMP_V2) is error-prone byte-twiddling with zero algorithmic-credibility value; gobgp gets it right, uses `net/netip`, and ships `SplitMrt` for `bufio.Scanner`. The GitHub story (valley-free, RPKI, RTR, synthetic generator) is all hand-rolled.

## Tech stack rationale
- **Go 1.26**: native concurrency for the streaming pipeline; `net/netip` for allocation-free prefix math; `log/slog`; `http.ServeMux` 1.22 pattern routing (no router dep). Rejected Python (the streaming/concurrency fit Go far better; this is a systems tool).
- **gobgp v4**: production MRT/BGP decoder. Rejected `kaorimatz/go-mrt` (unmaintained, no netip).
- **`github.com/coder/websocket`**: context-aware, no global state, clean shutdown. Rejected `gorilla/websocket` (heavier ctx story; acceptable fallback).
- **React 19 + Vite + TypeScript strict**: component model for chrome.
- **D3 `d3-force` for layout + Canvas 2D for the graph**: SVG node-per-circle forces React reconciliation every tick (jank at hundreds of nodes). Canvas draws the whole graph in one rAF with no DOM churn. D3 owns ONLY the simulation math; React/SVG only for the few interactive overlays (tooltip, selection ring, focus). Rejected pure-SVG (perf) and pure-DOM (impossible at scale).
- **Zustand** for client state (updates outside React's render cycle; selector subscriptions). Rejected Context (re-renders every consumer at 100+ ev/s). No TanStack Query (this is a push stream, not request/response).

---

# 3d. Feature list

## Core (must all be fully implemented — these are the project)
1. **MRT/BGP ingestor** — parses real RouteViews/RIPE RIS MRT dump files (TABLE_DUMP_V2 + BGP4MP/BGP4MP_ET MESSAGE/MESSAGE_AS4) via gobgp, streaming UPDATE/WITHDRAW with full AS_PATH (AS_SET + 4-byte ASNs), NEXT_HOP, communities; replays at a paced speed. Plus a deterministic synthetic generator over the same normalized event for offline demo.
2. **Gao-Rexford valley-free leak classifier** — loads CAIDA AS-relationship data (or the synthetic topology), tags each AS pair p2c/c2p/p2p/s2s, walks each path in propagation order through a two-phase (Up→Down) machine enforcing `(c2p)* (p2p|p2c)? (p2c)*`, flags valleys as leaks and pinpoints the offending AS hop.
3. **RPKI origin validation (RFC 6811)** — a prefix trie over VRPs (v4+v6); each (prefix, origin) stamped Valid / Invalid / NotFound with correct maxLength semantics; per-ASN throughput sparklines in the sidebar. VRPs from a bundled Routinator/rpki-client JSON export by default, or a live **RFC 8210 RTR** session.
4. **D3 force-directed AS topology** — nodes = ASNs sized (sqrt) by announced prefix count; edges colored by valley-free status (violet normal, amber leak, crimson hijack); offending hop ringed/pulsed; clicking an edge opens a drawer with the raw UPDATE/WITHDRAW events that produced it.

## Stretch (only if time allows)
- Live RIPE RIS streaming source (websocket to ris-live). v1 ships synthetic + MRT-file replay; the RTR client IS implemented (real, tested) for live VRPs.
- AS-org sibling enrichment from CAIDA as-org data.
- Time-scrubber to replay the last N seconds.

---

# 3e. File & folder structure

```
bgpulse/
├── CLAUDE.md                  # this file (the spec)
├── state.md                   # session state
├── README.md                  # comprehensive, with live screenshots
├── LICENSE                    # MIT
├── .gitignore
├── docker-compose.yml         # full stack: backend + frontend (nginx)
├── docs/
│   ├── ARCHITECTURE.md        # the reconciled architecture (reference)
│   └── screenshots/           # numbered live Playwright captures for README
├── backend/
│   ├── go.mod                 # module github.com/rayancheca/bgpulse/backend (go 1.26)
│   ├── Makefile               # build, test, test-race, cover, run-demo
│   ├── Dockerfile             # multi-stage static build
│   ├── cmd/bgpulse/main.go    # entrypoint: config, signal ctx, build+run server
│   ├── internal/
│   │   ├── bgp/               # lowest-level domain types; imports nothing internal
│   │   │   ├── types.go       # UpdateEvent, ClassifiedEvent, Community, PathHop, enums + String()
│   │   │   ├── source.go      # Source interface
│   │   │   ├── classifier.go  # Classifier interface
│   │   │   └── types_test.go
│   │   ├── relationships/     # AS-relationship store
│   │   │   ├── store.go       # RelStore: Lookup(a,b) RelStatus; immutable; builder
│   │   │   ├── caida.go       # CAIDA serial-2 "AS1|AS2|REL" loader
│   │   │   ├── names.go       # optional ASN->name table
│   │   │   └── *_test.go      # symmetry + parse tests
│   │   ├── valleyfree/        # Gao-Rexford valley-free classifier (REAL algorithm)
│   │   │   ├── classifier.go  # ClassifyPath(path []uint32, rel RelStore) (VFStatus,[]PathHop,offender)
│   │   │   └── classifier_test.go  # the >=12 case table
│   │   ├── rpki/              # RFC 6811 validation
│   │   │   ├── vrp.go         # VRP, ValidationState
│   │   │   ├── trie.go        # v4/v6 prefix trie
│   │   │   ├── validate.go    # RFC 6811 Validate (covering = containment only)
│   │   │   ├── jsonload.go    # Routinator/rpki-client JSON loader
│   │   │   └── *_test.go
│   │   ├── rtr/               # RFC 8210 RPKI-to-Router client
│   │   │   ├── pdu.go         # PDU type constants + wire structs
│   │   │   ├── codec.go       # encode/decode (big-endian)
│   │   │   ├── client.go      # session state machine
│   │   │   └── *_test.go      # codec round-trip + net.Pipe fake-server
│   │   ├── classify/          # combine valley-free + RPKI -> EventStatus (precedence)
│   │   │   ├── classify.go    # the concrete bgp.Classifier
│   │   │   └── classify_test.go
│   │   ├── synth/             # deterministic synthetic stream (DEMO default)
│   │   │   ├── topology.go    # ONE canonical tiered topology: ASNs, rels, prefix-owner map
│   │   │   ├── generator.go   # implements bgp.Source; seeded PCG; valid baseline traffic
│   │   │   ├── scenarios.go   # scheduled leak + hijack injection
│   │   │   ├── derive.go      # build RelStore + VRPStore from the topology (so demo lights up)
│   │   │   └── *_test.go      # determinism golden + integration (leak+hijack detected)
│   │   ├── mrt/               # MRT/BGP dump parsing via gobgp
│   │   │   ├── parser.go      # MRTMessage -> []UpdateEvent normalization
│   │   │   ├── reader.go      # gzip/bz2-aware framing (SplitMrt scanner)
│   │   │   ├── replay.go      # implements bgp.Source: paced file replay
│   │   │   └── *_test.go      # golden-file decode against bundled fixture
│   │   ├── topology/          # in-memory graph + single-writer aggregator
│   │   │   ├── graph.go       # TopologyGraph, ASNode, Edge, EdgeKey, ribEntry
│   │   │   ├── aggregator.go  # Aggregator actor: Run loop, snapReq handling
│   │   │   ├── apply.go       # the single mutation path (§3.3)
│   │   │   ├── ring.go        # EventRing (bounded)
│   │   │   ├── series.go      # ringCounters / SparkBucket for sparklines
│   │   │   ├── stats.go       # Stats + EWMA rate
│   │   │   └── *_test.go
│   │   ├── wshub/             # WebSocket fan-out
│   │   │   ├── hub.go         # register/unregister/broadcast; Run(ctx)
│   │   │   ├── client.go      # send chan, read/write pumps, drop-oldest
│   │   │   └── hub_test.go
│   │   ├── api/               # HTTP + WS handlers + DTOs (the wire contract)
│   │   │   ├── dto.go         # all JSON DTOs (the single source of truth)
│   │   │   ├── mapper.go      # bgp/topology -> DTO (enum->string)
│   │   │   ├── envelope.go    # Envelope[T], writeJSON, writeError
│   │   │   ├── rest.go        # REST handlers
│   │   │   ├── ws.go          # /ws upgrade -> hub
│   │   │   ├── middleware.go  # recoverer, requestLogger, cors
│   │   │   ├── router.go      # http.ServeMux wiring
│   │   │   └── *_test.go      # httptest contract tests + golden JSON fixtures
│   │   ├── pipeline/          # wires Source -> Classifier -> Aggregator -> Hub
│   │   │   ├── pipeline.go
│   │   │   └── pipeline_test.go  # synth -> classified -> aggregator; goleak; clean shutdown
│   │   ├── server/            # composition root
│   │   │   ├── server.go      # New(cfg,log): build everything; Run(ctx); http.Server + errgroup
│   │   │   └── sources.go     # selectSource(cfg): demo|replay with degrade logic
│   │   └── config/
│   │       ├── config.go      # Config, Load(), flag/env parse + validate
│   │       ├── limits.go      # all tuning constants (one place)
│   │       └── *_test.go
│   └── data/                  # bundled offline data (go:embed)
│       ├── embed.go
│       ├── demo_vrps.json     # VRP set covering synthetic prefixes (incl. craftable Invalids)
│       ├── as-rel.sample.txt  # trimmed real CAIDA subset for replay mode
│       ├── as-names.sample.txt
│       └── updates.sample.mrt # small real MRT dump for replay + parser golden test
└── frontend/
    ├── package.json
    ├── vite.config.ts
    ├── tsconfig.json          # strict: true
    ├── index.html
    ├── Dockerfile             # build -> nginx static serve
    ├── nginx.conf             # serve SPA + proxy /api and /ws to backend
    └── src/
        ├── main.tsx
        ├── app/
        │   ├── App.tsx                # shell
        │   ├── AppShell.tsx           # overlay layout docking floating panels
        │   └── providers.tsx          # reduced-motion, store hydration, WS connect
        ├── components/
        │   ├── topology/{TopologyCanvas.tsx,canvasRenderer.ts,hitTest.ts,TopologyOverlay.tsx,NodeTooltip.tsx,topology.css}
        │   ├── event-stream/{EventRail.tsx,EventRow.tsx,AsPath.tsx,event-stream.css}
        │   ├── rpki-sidebar/{RpkiSidebar.tsx,RpkiAsnCard.tsx,Sparkline.tsx,RpkiStamp.tsx,rpki-sidebar.css}
        │   ├── drill-down/{EdgeDrawer.tsx,EdgeVerdict.tsx,RawEventList.tsx,drill-down.css}
        │   ├── status-bar/{StatusBar.tsx,ModePill.tsx,EventRateMeter.tsx,IncidentCounters.tsx,status-bar.css}
        │   └── ui/{Panel.tsx,StatusDot.tsx,Badge.tsx,VisuallyHidden.tsx,ui.css}
        ├── hooks/{useWebSocket.ts,useTopologyStore.ts,useStreamStore.ts,useReducedMotion.ts,useResizeObserver.ts}
        ├── lib/{types.ts,schema.ts,forceGraph.ts,graphStore.ts,scales.ts,status.ts,format.ts,constants.ts}
        └── styles/{tokens.css,typography.css,global.css}
```

---

# Canonical domain types & algorithms (the correctness core)

## Enums (`internal/bgp/types.go`)
```go
type UpdateKind uint8        // KindAnnounce | KindWithdraw  -> "announce"|"withdraw"
type RelStatus uint8         // RelUnknown|RelCustomer|RelProvider|RelPeer|RelSibling
                             //   Lookup(a,b): a's view of b. customer=b is a's customer (a->b downhill)
                             //   provider = b is a's provider (a->b uphill); peer; sibling
type VFStatus uint8          // VFValid|VFLeak|VFHijack|VFUnknown -> "valid"|"leak"|"hijack"|"unknown"
type RPKIStatus uint8        // RPKINotFound|RPKIValid|RPKIInvalid -> "notfound"|"valid"|"invalid"
```

## UpdateEvent / ClassifiedEvent
`UpdateEvent{ ID EventID; Seq uint64; Timestamp time.Time; Kind UpdateKind; Prefix netip.Prefix; PeerAS uint32; ASPath []uint32 (left=collector neighbor … right=origin); HasASSet bool; NextHop netip.Addr; Communities []Community; OriginAS uint32 }`. Immutable after construction.
`ClassifiedEvent{ Event UpdateEvent; VFStatus VFStatus; RPKIStatus RPKIStatus; Hops []PathHop; OffenderAS uint32; Reason string }` — embeds by value.
`PathHop{ From, To uint32; Rel RelStatus; IsOffender bool }` in **wire order** (From=toward collector, To=toward origin) so the frontend indexes its wire-order `asPath` directly.

## Gao-Rexford valley-free walk (`internal/valleyfree`)
**Direction:** the wire AS_PATH is collector-first, origin-last. Propagation flows origin→collector (right→left). The classifier **reverses to propagation order** then walks left(origin)→right(collector). When stepping from `u` (nearer origin) to `v` (nearer collector) the relevant relationship is `Rel(u,v)` ("what is u to v?").

Steps:
1. Build propagation-ordered hop list: reverse the wire path; **collapse consecutive equal ASNs** (prepend dedup). Each AS_SET segment = a single opaque, relationship-transparent hop recorded in `setIdx` (set `HadASSet=true`); if the origin segment is an AS_SET the origin is unverifiable.
2. `len(hops) <= 1` → valley-free (no inter-AS move).
3. `phase := Up`. Walk pairs `(u,v)`:
   - either index in `setIdx` → `RelUnknown` for this move: no phase change, never flag, `HadUnknown=true`, continue.
   - `Rel(u,v)`:
     - `Sibling` → phase-transparent, continue.
     - `Unknown` → `HadUnknown=true`, **never flagged**, no phase change, continue.
     - `Customer→Provider` (uphill, i.e. u is customer of v): if `Up` stay Up; if `Down` → **VALLEY** (leak), offender = `(u,v)`, reason `uphill-after-descent-or-peak`.
     - `Peer`: if `Up` → transition to `Down` (the one allowed peer); if `Down` → **VALLEY**, reason `second-peer-or-peer-after-descent`.
     - `Provider→Customer` (downhill): always legal → transition/stay `Down`.
4. No violation → valley-free.

This exactly accepts `(c2p)* (p2p|p2c)? (p2c)*`. **Unknown and AS_SET links are never flagged** (no false leaks); siblings transparent. `ClassifyPath` returns `(VFStatus, []PathHop, offenderAS)` where offenderAS is the concrete `To`-side AS of the offending move (direction-agnostic on the wire).

## RFC 6811 RPKI validation (`internal/rpki`) — CORRECTED maxLength semantics
VRP = `{Prefix netip.Prefix (masked); MaxLength uint8 (Prefix.Bits()..familyMax); OriginAS uint32}`. AS0 = disavow.
Store: separate v4/v6 binary tries keyed on address bits; node at depth d holds VRPs whose prefix length == d.

`Validate(prefix, origin)`:
1. Pick trie by family. Let `pBits = prefix.Bits()`. Descend the trie following the announced prefix's bits for depths `0..pBits` (inclusive). Every VRP encountered along this descent **COVERS** the announcement (its prefix contains the announced prefix and `VRP.Bits() <= pBits`). **maxLength is NOT part of covering.**
2. For each covering VRP V: `CoveringVRPs++`. It **MATCHES** iff `pBits <= V.MaxLength` AND `V.OriginAS == origin` AND `origin != 0` AND `V.OriginAS != 0`.
3. Decide: any match → **Valid**; else if `CoveringVRPs > 0` → **Invalid**; else → **NotFound**.

> CRITICAL: a route contained by a covering VRP but more-specific than its `maxLength`, with no matching VRP, is **Invalid** (NOT NotFound). This is the headline RFC 6811 subtlety and it is what catches more-specific hijacks.

## Classification precedence (`internal/classify`)
`HIJACK > LEAK > NORMAL`. If `RPKIStatus == Invalid` → `VFHijack`. Else if valley → `VFLeak`. Else `VFValid` (or `VFUnknown` if relationships were insufficient AND not a leak). Retain both sub-results for the UI ("hijack — also leaks"). If origin came from an AS_SET / empty path, RPKI is NotFound + advisory (no RPKI-driven hijack).

## ONE canonical synthetic topology (`internal/synth`)
A single deterministic tiered graph is the source of truth for demo mode, consumed by the generator AND derived into the RelStore AND the VRP set:
- Tier-1 ASNs `1001..1006` fully meshed as **peers**.
- Transit ASNs `2001..2024`, each a **customer** of 1–2 tier-1s; transit↔transit peer with prob 0.15.
- Stub ASNs `3001..3120`, each a **customer** of 1 transit (+ second w/ prob 0.30); every 17th stub + successor are **siblings**.
- Each stub/transit **owns** one or more prefixes (deterministic CIDR allocation); `demo_vrps.json` / the derived VRP set authorizes those (prefix→ownerAS, maxLength).
- Generator emits a **valid valley-free baseline** (paths built by walking up the customer cone, optionally one peer, down) at a paced rate; **injects a leak** every ~30 virtual-sec (a customer/peer re-announces a provider/peer route up to another provider → catchable valley) and a **hijack** every ~45 virtual-sec (announce a prefix from the wrong origin → RPKI Invalid, and/or a more-specific of an owned prefix). `math/rand/v2` PCG, `uint64` seed (default 42), virtual clock from a fixed base time. **Integration test** asserts the injected leak is `VFLeak` and the injected hijack is `RPKIInvalid`/`VFHijack` end-to-end.

## RFC 8210 RTR client (`internal/rtr`)
Protocol v1. 8-byte header `Version|Type|Session|Length` big-endian. PDUs: SerialNotify(0), SerialQuery(1), ResetQuery(2), CacheResponse(3), IPv4Prefix(4), IPv6Prefix(6), EndOfData(7), CacheReset(8), RouterKey(9, skip), ErrorReport(10). State machine: connect → ResetQuery → CacheResponse → Prefix PDUs → EndOfData (record session+serial) → established; on SerialNotify send SerialQuery for deltas; on CacheReset full resync; session-id mismatch → reset. VRPs swapped into the validator via `atomic.Pointer[VRPStore]`. Offline default: not started — `LoadVRPsJSON` installs a static store via the same pointer. Tested with codec round-trips + a `net.Pipe` fake cache.

---

# Wire contract (the single source of truth — Go `json` tags ↔ TS types)

WS frame envelope: `{ "type": "snapshot"|"event"|"stats", "seq": uint64, <payload> }`.
- `event` payload = `ClassifiedDTO`. `snapshot` = `{topology, events[], stats}`. `stats` = `StatsDTO` (periodic, ~1/s wall).
- Timestamps: **RFC3339 strings**. Enums: lowercase machine tokens (UI maps to display labels: rpki `valid`→"Valid" etc.). Communities: `{asn,value}` objects. Edges: **directed** (`from`,`to`).

```
ClassifiedDTO { id, seq, timestamp, kind("announce"|"withdraw"), prefix(CIDR), peerAs,
                asPath[uint32] (left=collector…right=origin), nextHop, communities[{asn,value}],
                originAs, vfStatus("valid"|"leak"|"hijack"|"unknown"),
                rpkiStatus("valid"|"invalid"|"notfound"), hops[{from,to,rel,isOffender}],
                offenderAs(uint32, 0 if none), reason }
NodeDTO { asn, name, prefixCount, rpki{valid,invalid,notfound}, firstSeen, lastSeen }
EdgeDTO { from, to, status, rel, count, leakCount, hijackCount, lastEvent, lastEventId }
TopologyDTO { nodes[], edges[], nodeCount, edgeCount, generated }
OriginStatDTO { asn, prefixCount, rpkiStatus, valid, invalid, notfound, throughput[int] }  // sidebar
StatsDTO { totalEvents, announces, withdraws, leaks, hijacks, rpkiValid, rpkiInvalid,
           rpkiNotFound, nodeCount, edgeCount, eventsPerSec, clientsConnected,
           topOrigins[OriginStatDTO] }
SnapshotDTO { topology TopologyDTO, events[ClassifiedDTO], stats StatsDTO }
HealthDTO { ok, mode, version, uptimeSec, sources{bgp,relationships,rpki,liveFellBack} }
ASNDetailDTO { node, neighbors[{asn,direction,rel,status}], prefixes[], sparkline[{start,valid,invalid,notfound,total}] }
EdgeDetailDTO { edge EdgeDTO, events[ClassifiedDTO] }
```
REST envelope: `{ "ok": bool, "data"?: T, "error"?: string }`. Endpoints: `GET /api/health`, `/api/topology`, `/api/events?limit=N`, `/api/asn/{asn}`, `/api/edge/{from}/{to}`, `/api/stats`, `GET /ws` (upgrade). A **golden-fixture contract test** validates the same JSON against the Go marshaller and the frontend zod schema.

---

# 3h. Visual direction — `routing-observatory-violet-constellation`

**Derived from:** routing-security/NOC engineers who live in looking glasses (bgp.he.net), RIPEstat/BGPlay, Kentik, Grafana — they trust monospace data tables and think in graphs. The data has a dual register: a calm, structural "shape of the internet" at rest, and an urgent incident klaxon when a leak/hijack fires. The metaphor: an **internet routing observatory** — a near-black backbone canvas (deep space), AS nodes as violet stars forming a constellation, anomalies as flares. Violet is promoted to BOTH brand accent and the route-normal status, so a healthy graph glows in its own color and anomalies (amber/crimson) read as intrusions. Distinct from the whole portfolio (no phosphor-green/seafoam baseline, no JetBrains+Inter, warm violet-tinted near-black not the cold `#0d1117`).

**Palette (CSS custom properties; OKLCH is the intent, hex is the value):**
```
--color-bg:#08070d  --color-surface:#100e18  --color-surface-elevated:#181527
--color-border:#272234  --color-border-strong:#3a3350
--color-text-primary:#ECE8F5  --color-text-secondary:#9B93B0  --color-text-tertiary:#645C78
--color-accent:#8B5CF6  --color-accent-bright:#A78BFA  --color-accent-dim:#5B3FA0  --color-accent-glow:#C4B5FD
--color-route-normal:#7C6CF0 (violet)  --color-route-leak:#F2A23C (amber)
--color-route-hijack:#F2415A (crimson)  --color-route-withdraw:#645C78 (gray)
--color-rpki-valid:#34D399 (emerald — ONLY green, reserved for "verified")
--color-rpki-invalid:#F2415A (crimson, == hijack: an Invalid origin IS the hijack signal)
--color-rpki-notfound:#8A8198 (slate, neutral)
```
**Fonts (self-hosted via @fontsource):** UI/display = **Space Grotesk** (geometric grotesque, instrument-panel feel); data/mono = **IBM Plex Mono** (every ASN/prefix/IP/AS_PATH/timestamp; humanist, tabular figures, disambiguated 0/O 1/l/I). Two families only, `font-display: swap`, preload critical weights.

**Layout — the graph IS the design (HUD, not dashboard):** full-viewport topology canvas; floating translucent (`backdrop-filter: blur(8px)`) overlays docked to edges: top **status bar** (mode pill + connection dot · ev/s · leak counter · hijack counter), bottom-left **event-stream rail** (newest-first), right **RPKI sidebar** (one card per active origin ASN, Invalid-first, with stamp + throughput sparkline), and an **edge drill-down drawer** sliding from the right on edge click.

**Interaction/motion (compositor-only `transform`/`opacity`):** edges rest at alpha 0.55 (normal) / 0.9 (anomaly); hover/focus → alpha 1.0 + endpoint halos + SVG highlight; selected edge → drawer opens, non-incident edges dim to 0.2. New node spawns scale `0→r` seeded at a neighbor (no corner fly-in). Leak → offending-hop amber ring pulses 6s then holds; hijack → sharp crimson flash on the bogus origin + counter pop. Full **reduced-motion** path: simulation runs to convergence off-screen, renders settled, static rings instead of pulses; all durations gated by a `--motion-scale` var. Color is never the sole signal (text `LEAK`/`HIJACK` tags; literal `Valid`/`Invalid`/`NotFound`). Contrast AA/AAA verified.

**Anti-checklist (must all be false):** looks like a Tailwind/shadcn template; uniform radius/spacing/shadow; accent is `#3b82f6`; no depth/hierarchy; browser-default hover/focus. → All false: bespoke violet observatory, floating layered HUD overlays, designed states.

---

# 3f. Implementation steps (strict order; commit + update state.md after each)

1. **Scaffold** — repo dirs, `backend/go.mod` (module `github.com/rayancheca/bgpulse/backend`, go 1.26), `backend/Makefile`, frontend Vite+React+TS strict skeleton, README skeleton, docker-compose stub. Verify `go build ./...` (empty ok) and `npm run build` skeleton. `chore: scaffold monorepo structure`.
2. **`internal/bgp` types** — all domain structs + enums + `String()` + immutability. Tests for enum strings. `go test ./internal/bgp`. `feat: bgp domain types and enums`.
3. **`internal/relationships`** — `RelStore` (builder + immutable Lookup with provider/customer inversion + sibling/self) + CAIDA serial-2 loader + names. Symmetry + parse tests. `feat: AS-relationship store + CAIDA loader`.
4. **`internal/valleyfree`** — the two-phase Gao-Rexford `ClassifyPath`. The ≥12-case table test (valid up/peer/down, every leak flavor, offender id, prepend dedup, unknown-never-flagged, AS_SET transparency). `feat: Gao-Rexford valley-free classifier`.
5. **`internal/rpki`** — VRP, v4/v6 trie, `Validate` (corrected covering=containment; maxLength→Invalid), JSON loader (AS-string + numeric ASN). Tests: Valid/Invalid/NotFound, maxLength boundary→Invalid, AS0→Invalid, IPv6, multi-covering. `feat: RFC 6811 RPKI origin validation`.
6. **`internal/classify`** — concrete `bgp.Classifier` combining valley-free + RPKI with `Hijack>Leak>Normal` precedence; builds `ClassifiedEvent` + `Hops` + offender + reason. Tests incl. precedence. `feat: event classifier with hijack>leak precedence`.
7. **`internal/synth`** — canonical tiered `topology.go`, `derive.go` (RelStore+VRPStore from topology), `generator.go` (paced valid baseline), `scenarios.go` (scheduled leak+hijack). Determinism golden test + **integration test** (synth→classify detects the injected leak and hijack). `feat: deterministic synthetic BGP stream generator`.
8. **`internal/mrt`** — gobgp-based `parser.go` (MRTMessage→[]UpdateEvent, bounds-check PEER_INDEX), `reader.go` (gzip/bz2 + SplitMrt), `replay.go` (paced `bgp.Source`). Golden decode test vs bundled `updates.sample.mrt`. `feat: MRT/BGP dump parser and replay source`.
9. **`internal/rtr`** — `pdu.go`, `codec.go` (round-trip), `client.go` (state machine). `net.Pipe` fake-server test. `feat: RFC 8210 RTR client`.
10. **`internal/topology`** — graph/node/edge/rib structs, `EventRing`, `series`, `Stats`+EWMA, `Aggregator` actor (`Run`, snapReq), `apply` (prefix-count in/out, origin-change, edge worst-status, self-loop skip, negative guard). `-race` tests. `feat: single-writer topology aggregator`.
11. **`internal/api`** — DTOs, mapper (enum→string), envelope, REST handlers, router, middleware. `httptest` contract tests + golden JSON. `feat: HTTP REST API + DTO contract`.
12. **`internal/wshub`** — Hub + Client + drop-oldest + disconnect-threshold + marshal-once. In-memory slow-client test. `feat: WebSocket fan-out hub`.
13. **`internal/config` + `internal/pipeline` + `internal/server` + `cmd/bgpulse/main.go`** — config flag/env+validate+degrade, limits, pipeline wiring (1 classifier worker), composition root, signal ctx, graceful shutdown, errgroup, goleak. Run `bgpulse -mode demo`, curl `/api/health` + `/api/topology`, confirm WS streams events. `feat: pipeline wiring, server, and demo entrypoint`.
14. **Frontend scaffold + design system** — Vite config, tsconfig strict, fonts (@fontsource Space Grotesk + IBM Plex Mono), `tokens.css` palette, `typography.css`, `global.css`. `feat: frontend design system + tokens`.
15. **Frontend lib** — `types.ts` (mirror DTOs exactly), `schema.ts` (zod discriminated union), `constants.ts`, `format.ts`, `status.ts`, `scales.ts`, `forceGraph.ts`, `graphStore.ts`. Vitest unit tests (reducers, scales, status, format, schema vs golden). `feat: frontend types, schemas, and graph reducers`.
16. **Frontend hooks** — `useWebSocket` (zod-validated, backoff reconnect), `useTopologyStore`, `useStreamStore`, `useReducedMotion`, `useResizeObserver`. `feat: frontend stores and websocket client`.
17. **Topology canvas** — `TopologyCanvas` + `canvasRenderer` (nodes/edges/rings/labels) + `hitTest` (quadtree + segment) + `TopologyOverlay`/`NodeTooltip`. `feat: D3 force-directed topology canvas`.
18. **Panels** — event rail (+AsPath offending-hop highlight), status bar (counters flash), RPKI sidebar (cards + sparkline + stamp), edge drawer (verdict + raw events). `feat: event rail, status bar, RPKI sidebar, drill-down drawer`.
19. **App shell + full-stack run** — wire `App`/`AppShell`/`providers`; run backend+frontend; Playwright drives golden path (connect→stream→leak amber→hijack crimson→click edge→drawer). Fix until clean. `feat: app shell + verified golden path`.
20. **Docker + README + screenshots + release** — backend/frontend Dockerfiles, `docker-compose.yml`, nginx proxy; comprehensive README; ≥6 live numbered screenshots in `docs/screenshots/`; final e2e; `docs/ARCHITECTURE.md`; mark state COMPLETE. `release: v1.0.0 — BGPulse complete`.

# 3g. README spec
Badges (Go version, license, build). Hero screenshot/GIF of the violet topology with a live leak. Sections: What it is (the routing-security problem) · Live workflow screenshots (≥6 numbered: empty→connect→steady-state graph→leak fires amber→hijack fires crimson→edge drill-down drawer) · Architecture (the pipeline diagram + single-writer rationale) · Technical deep-dive (Gao-Rexford valley-free walk, RFC 6811 corrected maxLength semantics, RFC 8210 RTR, deterministic generator) · Install & run (`docker compose up`, and dev: backend `make run-demo`, frontend `npm run dev`) · Usage (real sample REST/WS output) · Replay-your-own-MRT (`-mode replay -mrt-file …`) · License (MIT).
