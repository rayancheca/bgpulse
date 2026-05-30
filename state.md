## Status
IN PROGRESS

## Project
bgpulse — Live BGP route-leak and prefix-hijack detector with AS-path topology visualization.

## Session count
1

## Completed steps
- STEP 3 — Full technical spec written (planning workflow: 4 expert design agents + adversarial audit; reconciled into CLAUDE.md as single source of truth). Audit fixes locked in: RFC 6811 covering=containment (maxLength→Invalid not NotFound); ONE canonical synthetic topology shared by generator+classifier+VRPs; pinned wire contract; single classifier goroutine (per-prefix order); math/rand/v2 PCG uint64 seed; directed edges + offender-as-concrete-ASN; scope = demo+replay+JSON-VRP+real RTR (live RIS deferred).
- Visual direction derived + recorded in project_history.md: routing-observatory-violet-constellation.
- STEP 1 — Scaffold: backend go.mod (module github.com/rayancheca/bgpulse/backend, go 1.26.2), backend/Makefile, frontend Vite+React19+TS (strict mode added: strict/noImplicitOverride/noUncheckedIndexedAccess), deps installed (d3, zustand, zod, @fontsource/space-grotesk, @fontsource/ibm-plex-mono, vitest+jsdom+testing-library), README skeleton, LICENSE, .gitignore. Verified: frontend `npm run build` OK (60kB gz), backend `go build ./...` OK.

- STEP 2 — internal/bgp: types.go (UpdateEvent, ClassifiedEvent, Community, PathHop), enums UpdateKind/RelStatus/VFStatus/RPKIStatus with String()/Invert()/Severity(), source.go (Source iface), classifier.go (Classifier iface). Tests green, go vet clean. RelStatus semantics: Lookup(a,b) = a's view of b; RelCustomer=b is a's customer (downhill), RelProvider=b is a's provider (uphill).

- STEP 3 — internal/relationships: RelStore (immutable, canonical (lo,hi) edges, Lookup with Invert + self=sibling + missing=unknown), Builder (Add normalizes + conflict-detects), CAIDA serial-2 loader (ParseCAIDA/LoadCAIDA: -1=>provider→RelCustomer, 0=>peer), names table (ParseNames). Tests 93.2% coverage, vet clean.

- STEP 4 — internal/valleyfree: ClassifyPath(path,hasASSet,rl)->Verdict{IsLeak,OffenderAS,Reason,Hops(wire order),HadUnknown,KnownHops}. Two-phase Up→Down machine; walks propagation order (decreasing wire idx); offender = leaking AS (To-side of offending wire hop); siblings transparent; unknown+AS_SET never flagged; prepend dedup. 13-case table test (uphill/downhill/peer/leak×3/sibling/unknown/AS_SET) at 97.7% coverage.

- STEP 5 — internal/rpki: VRP, v4/v6 binary trie (VRPStore immutable, Builder), Validate (covering=containment via trie descent; match needs pBits<=maxLength && origin==VRP.origin && origin!=0; Valid>Invalid>NotFound) — maxLength-exceeded+no-match => Invalid (the RFC 6811 fix). AS0 disavow → Invalid. LoadVRPsJSON (AS-string + numeric asn, default maxLength). 14-case validate table + loader tests at 96.5% coverage.

- STEP 6 — internal/classify: Classifier (New(rel,vrp Validator); Validator iface so RTR can swap). Classify: withdraw=>neutral; else valleyfree+RPKI; precedence Hijack(origin offender)>Leak(vf offender)>Unknown(knownHops==0)>Valid. 100% coverage.

- STEP 7 — internal/synth: BuildDefault(seed)->Topology (tier1 1001-1006 full peer mesh; transit 2001-2024, AS2001 forced multi-homed to 1001+1002 for the leak scenario; stubs 3001-3120; prefixes in 11/12/100/101 blocks; ~80% ROA'd) deriving RelStore+VRPStore from ONE source. Generator (bgp.Source, math/rand/v2 PCG DefaultSeed, virtual clock baseTime 2024-01-01; Next()+Events(); validPath = up-peak-down valley-free; makeLeak wire [1002,2001,1001] offender 2001; makeHijack stub announces a victim ROA'd prefix → RPKI Invalid). Tests: determinism golden + integration (600 events → 50 leaks/29 hijacks, ZERO false positives, every leak offender=2001, every hijack Invalid-from-stub). 86.1% coverage. Full suite green with -race.

## BACKEND CORE COMPLETE (steps 1-7). Pure algorithmic layer done + tested. Coverage: relationships 93%, valleyfree 98%, rpki 96%, classify 100%, synth 86%.

- STEP 8 — internal/mrt: gobgp v4.5.0 added (only 3rd-party dep). parser.go (recordToEvents: BGP4MP MESSAGE/AS4 → []UpdateEvent; AS_PATH flatten, AS_SET→single opaque hop+HasASSet, origin=last SEQ elem/0 if trailing SET, communities, v4 NLRI/withdraw + v6 MP_REACH/MP_UNREACH). reader.go (DecodeStream via SplitMrt; OpenFile gzip/bz2). replay.go (ReplaySource bgp.Source, paced, optional loop, assigns mrt-NNN ids). sample.go (BuildSampleMRT real BGP4MP bytes). cmd/gen-mrt writes data/updates.sample.mrt (334B, committed; .gitignore exception added). Golden test: 4 records decode correctly (announce/AS_SET-origin0/v6/withdraw). Full suite -race green.

- STEP 9 — internal/rtr: RFC 8210 v1 client. pdu.go (PDU type consts, error codes, decoded structs). codec.go (readPDU decode; EncodePrefix/EndOfData/SerialNotify/CacheResponse/CacheReset + writeResetQuery/writeSerialQuery). client.go (Client.Run: dial→ResetQuery full sync→CacheResponse/Prefix/EndOfData→commit; SerialNotify or refresh-deadline→SerialQuery delta; CacheReset→resync; ErrorReport fatal/transient; reconnect w/ backoff; context.AfterFunc closes conn on cancel; commits via rpki.Live atomic swap). Also added rpki/live.go (atomic-swappable Validator). Tests: codec round-trips + net.Pipe full-sync+delta integration, -race green.

- STEP 10 — internal/topology: single-writer Aggregator actor (Run select loop over in/snapReqs/ctx/1s-rate-ticker; no locks). graph.go (TopologyGraph Nodes/Edges directed/rib, ASNode w/ prefix set+RPKICounts+series, Edge). ring.go (EventRing bounded, recent/byEdge). series.go (ringCounters virtual-time sparkline buckets, bounded advance). stats.go (counters + EWMA tick). apply.go (origin RPKI tally+series, edge upsert showing LATEST status+cumulative leak/hijack counts, RIB prefix-count in/out + origin-change, self-loop skip, AS_SET-origin not tracked). views.go (NodeView/EdgeView/SnapshotView/StatsView+TopOrigins/ASNDetailView/EdgeDetailView/FullSnapshot — topology returns its own view types, api maps to DTO to avoid cycle). aggregator.go (read methods via snapReq+reply, broadcast non-blocking to out chan). Tests: direct apply unit tests + concurrent actor (synth+classifier, 3 reader goroutines) -race green, 89.2% coverage. NOTE: api maps topology views→DTO; aggregator broadcasts bgp.ClassifiedEvent on out chan (server maps+marshals for hub) — NO topology→api/wshub import.

- STEP 11 — internal/api: dto.go (full pinned wire contract incl WSMessage type snapshot/event/stats), mapper.go (topology views + ClassifiedEvent → DTO; enum .String() lowercase tokens, RFC3339, {asn,value}), envelope.go (Envelope[T]/writeOK/writeErr), server.go (Store iface — *topology.Aggregator satisfies it; HealthInfo; Routes(wsHandler) with recoverer/logger/cors), rest.go (handlers + parseASN/parseLimit). Tests: httptest status codes + byte-exact golden ClassifiedDTO JSON. api imports topology+bgp (no cycle).
- STEP 12 — internal/wshub: coder/websocket v1.8.14. hub.go (single-goroutine Run owns clients; sendToClient drop-oldest + disconnect>512 drops; Broadcast non-blocking; Handler upgrades, writes snapshot directly pre-pump, registers; h.done prevents unregister leak on shutdown). client.go (writePump drains send→conn, readPump detects close→unregister). Tests: drop-oldest+disconnect unit + real WS dial snapshot+broadcast, -race green.

- STEP 13 — config+pipeline+server+main: internal/config (Load flags+env+validate, NewLogger, limits; config_test: defaults/flags/env precedence/5 validation errors). internal/pipeline (Source→1 classifier goroutine→aggregator.in, owns+closes chan). api/frames.go (EventFrame/StatsFrame/SnapshotFrame). internal/server (buildSources mode demo|replay+bundled-fallback+optional RTR; New wires all; Run errgroup + graceful HTTP shutdown, swallows context.Canceled on clean SIGTERM; staticFallback for single-binary frontend). data/embed.go + bundled as-rel.sample.txt + demo_vrps.json. cmd/bgpulse/main.go. VERIFIED LIVE: demo → 4s: 65 events/5 leaks/3 hijacks/73 nodes/109 edges/~14ev/s/40 topOrigins; health/stats/topology/asn all correct; clean SIGTERM exit 0. -race green.

## ✅✅ BACKEND COMPLETE (steps 1-13). Runs end-to-end. Core coverage: relationships 93% valleyfree 98% rpki 96% classify 100% synth 86% topology 89%. Build: `go -C backend build ./...`; run: `./backend/bin/bgpulse -mode demo` (REST+WS on :8080).

## In progress
STEP 14 (frontend design system) — tokens.css (routing-observatory violet palette + type scale), typography.css (@fontsource Space Grotesk + IBM Plex Mono), global.css (reset, focus-visible, scrollbar), vitest config. Verify npm build.

## Next steps (CLAUDE.md §3f)
14. Frontend design system + tokens. Verify npm build.
15. lib/: types.ts (mirror DTOs exactly), schema.ts (zod), constants/format/status/scales/forceGraph/graphStore + vitest.
16. hooks/: useWebSocket (zod+backoff), useTopologyStore, useStreamStore, useReducedMotion, useResizeObserver.
17. components/topology: TopologyCanvas + canvasRenderer + hitTest + overlay (Canvas2D + d3-force).
18. components/: event-stream rail, status-bar, rpki-sidebar, drill-down drawer.
19. app shell + providers; run full stack; Playwright golden path; fix until clean.
20. Dockerfiles + docker-compose + nginx; README + >=6 live screenshots; release v1.0.0.

## WIRE CONTRACT for frontend (pinned, from api/dto.go + golden in api/rest_test.go):
WS frame: {type:"snapshot"|"event"|"stats", seq, event?|stats?|snapshot?}. event payload=ClassifiedDTO{id,seq,timestamp(RFC3339 string),kind:"announce"|"withdraw",prefix(CIDR),peerAs,asPath[uint32](left=collector..right=origin),nextHop,communities[{asn,value}],originAs,vfStatus:"valid"|"leak"|"hijack"|"unknown",rpkiStatus:"valid"|"invalid"|"notfound",hops[{from,to,rel,isOffender}],offenderAs(0=none),reason}. stats=StatsDTO{totalEvents,announces,withdraws,leaks,hijacks,rpkiValid,rpkiInvalid,rpkiNotFound,nodeCount,edgeCount,eventsPerSec,topOrigins[{asn,name,prefixCount,rpkiStatus,valid,invalid,notfound,throughput[int]}]}. snapshot={topology{nodes[{asn,name,prefixCount,rpki{valid,invalid,notfound},firstSeen,lastSeen}],edges[{from,to,status,rel,count,leakCount,hijackCount,lastEvent,lastEventId}],nodeCount,edgeCount,generated},events[ClassifiedDTO],stats}. REST envelope {ok,data?,error?}. Dev: backend :8080 (set -cors-origin http://localhost:5173), vite :5173, VITE_WS_URL=ws://localhost:8080/ws. UI label: capitalize rpki wire token → Valid/Invalid/NotFound.
12. internal/wshub Hub + Client drop-oldest + slow-client test.
13. internal/config + pipeline + server + cmd/bgpulse/main.go; run demo, curl, confirm WS.
14-20. Frontend + docker/readme/screenshots.
11. internal/api DTOs + mapper + envelope + REST + golden JSON contract test.
12. internal/wshub Hub + Client drop-oldest + slow-client test.
13. internal/config + pipeline + server + cmd/bgpulse/main.go; run demo, curl health/topology, confirm WS.
14-19. Frontend (design system, lib, hooks, canvas, panels, app shell + Playwright golden path).
20. Docker compose + README + live screenshots + release.

## Blockers
None.

## Notes
- Module path: github.com/rayancheca/bgpulse/backend. GitHub repo: github.com/rayancheca/bgpulse (empty, remote set).
- Toolchain: Go 1.26.2, Node 25, pnpm 10.33, Docker 29.3.1. gh authed as rayancheca.
- MRT/BGP wire decode uses gobgp v4 (github.com/osrg/gobgp/v4/pkg/packet/{mrt,bgp}); everything above it (valley-free, rpki, rtr, synth) is hand-rolled — that's the GitHub story.
- Full design docs (4 sections) saved in /tmp this session: bgp_dataplane.md, bgp_algorithms.md, bgp_backendArch.md, bgp_frontendVisual.md, bgp_audit.json. Reconciled spec is CLAUDE.md (the authority — design docs had inconsistencies the audit caught).
- Session strategy: this session aim to land the pure-logic backend core (steps 1–7, the highest-value algorithms) with full tests via `go test`.

## Git log
No commits yet.
