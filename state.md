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

## In progress
STEP 6 (internal/classify) — concrete bgp.Classifier combining valleyfree+rpki with Hijack>Leak>Normal precedence; builds ClassifiedEvent (VFStatus/RPKIStatus/Hops/OffenderAS/Reason); origin-from-AS_SET/empty => RPKI advisory.

## Next steps (implementation plan in CLAUDE.md §3f, strict order)
6. internal/classify precedence glue + tests.
4. internal/valleyfree Gao-Rexford two-phase ClassifyPath + >=12-case table test.
5. internal/rpki VRP trie + RFC 6811 Validate (corrected maxLength) + JSON loader + tests.
6. internal/classify precedence glue + tests.
7. internal/synth canonical topology + generator + scenarios + derive + determinism + integration test.
(then mrt, rtr, topology, api, wshub, config/pipeline/server/main, then frontend, then docker/readme/screenshots — see CLAUDE.md §3f steps 8–20)

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
