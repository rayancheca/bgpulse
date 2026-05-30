<h1 align="center">BGPulse</h1>

<p align="center">
  <strong>Live BGP route-leak and prefix-hijack detector with AS-path topology visualization.</strong>
</p>

<p align="center">
  <img alt="Go" src="https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white">
  <img alt="TypeScript" src="https://img.shields.io/badge/TypeScript-strict-3178C6?logo=typescript&logoColor=white">
  <img alt="React" src="https://img.shields.io/badge/React-19-149ECA?logo=react&logoColor=white">
  <img alt="License" src="https://img.shields.io/badge/license-MIT-8B5CF6">
</p>

---

## What it is

BGP route leaks and prefix hijacks — the failure mode behind the Facebook 2021 outage and
repeated AWS/Cloudflare incidents — propagate across the internet for minutes before operators
notice, and most tooling gives no real-time visual signal of *why* a route is wrong.

**BGPulse** ingests a stream of BGP UPDATE/WITHDRAW messages, runs two of the algorithms at the
heart of internet routing security against every announcement in real time —

- the **Gao-Rexford valley-free** model, to detect route **leaks** in the AS_PATH topology, and
- **RFC 6811 RPKI Route Origin Validation**, to detect prefix **hijacks** at the origin —

and renders the result as an interactive, force-directed **AS-path topology** where every anomaly
lights up the moment it propagates through the routing table.

It runs **fully offline** out of the box on a deterministic synthetic BGP stream that injects real
leaks and hijacks on a schedule, and it will also replay real RouteViews/RIPE RIS MRT dumps and
validate against a live Routinator RTR session.

## Architecture (at a glance)

```
Source ──▶ Classifier ──▶ Aggregator (single-writer) ──▶ WebSocket Hub ──▶ React + D3 topology
(synthetic │ (Gao-Rexford   │ (in-memory AS graph,         │ (drop-oldest
 or MRT    │  valley-free + │  RIB, event ring,            │  per client)
 replay)   │  RFC 6811 RPKI)│  RPKI sparklines)            │
```

- **Backend** — Go 1.26: a streaming pipeline with a single-writer "actor" owning the AS graph
  (no mutex, race-free by construction), MRT/BGP wire decoding via gobgp, a hand-rolled valley-free
  classifier, an RFC 6811 RPKI trie, and an RFC 8210 RTR client.
- **Frontend** — React 19 + TypeScript + D3 `d3-force` for layout + Canvas 2D for the graph,
  designed as an "internet routing observatory": a violet AS constellation on a near-black backbone
  that ignites amber on a leak and crimson on a hijack.

## Status

Under active construction — see [`state.md`](state.md) for the live build log and
[`CLAUDE.md`](CLAUDE.md) for the full technical specification. The README will grow a complete
walkthrough with live screenshots, install/run instructions, and a technical deep-dive as the
build progresses.

## License

[MIT](LICENSE)
