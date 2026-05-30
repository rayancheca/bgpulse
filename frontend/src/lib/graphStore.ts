// Pure reducers that maintain the live AS graph (nodes + directed edges) from the
// three server frame types. They mutate the Maps and the SimNode/SimLink objects in
// place — the sanctioned simulation-layer mutation — so d3-force preserves node
// positions across upserts. Each reducer reports whether the graph STRUCTURE changed
// (a new node or edge), which tells the canvas to rebind and reheat the simulation.
import type { ClassifiedEvent, Relationship, SimLink, SimNode, Stats, Topology } from './types'
import { radiusFor } from './scales'
import { rpkiFromCounts } from './status'

export interface GraphState {
  nodes: Map<number, SimNode>
  links: Map<string, SimLink>
  maxPrefixCount: number
}

export function createGraphState(): GraphState {
  return { nodes: new Map(), links: new Map(), maxPrefixCount: 1 }
}

// edgeKey identifies a directed adjacency, matching the backend's directed edges.
export function edgeKey(from: number, to: number): string {
  return `${from}>${to}`
}

function newNode(asn: number, clock: number, max: number, near?: SimNode): SimNode {
  const n: SimNode = {
    asn,
    name: '',
    prefixCount: 1,
    rpki: 'notfound',
    offenderAt: 0,
    offenderStatus: 'valid',
    radius: radiusFor(1, max),
    bornAt: clock,
  }
  // Seed a new node near a neighbour so it grows in place instead of flying in.
  if (near && near.x != null && near.y != null) {
    n.x = near.x + (Math.random() - 0.5) * 24
    n.y = near.y + (Math.random() - 0.5) * 24
  }
  return n
}

function relForHop(ev: ClassifiedEvent, from: number, to: number): Relationship {
  for (const h of ev.hops) {
    if (h.from === from && h.to === to) return h.rel
  }
  return 'unknown'
}

// applyEvent folds one classified event into the graph.
export function applyEvent(g: GraphState, ev: ClassifiedEvent, clock: number): boolean {
  if (ev.kind !== 'announce') {
    return false // withdraws carry no path; they surface only in the event rail
  }
  let structureChanged = false

  let prev: SimNode | undefined
  for (const asn of ev.asPath) {
    let n = g.nodes.get(asn)
    if (!n) {
      n = newNode(asn, clock, g.maxPrefixCount, prev)
      g.nodes.set(asn, n)
      structureChanged = true
    }
    prev = n
  }

  for (let i = 0; i + 1 < ev.asPath.length; i++) {
    const from = ev.asPath[i]
    const to = ev.asPath[i + 1]
    if (from === undefined || to === undefined || from === to) {
      continue
    }
    const key = edgeKey(from, to)
    let link = g.links.get(key)
    if (!link) {
      link = {
        source: from,
        target: to,
        edgeKey: key,
        from,
        to,
        rel: relForHop(ev, from, to),
        status: ev.vfStatus,
        count: 0,
        leakCount: 0,
        hijackCount: 0,
        lastSeq: ev.seq,
        bornAt: clock,
        touchedAt: clock,
        withdrawnAt: null,
      }
      g.links.set(key, link)
      structureChanged = true
    }
    link.count++
    if (ev.vfStatus === 'leak') link.leakCount++
    else if (ev.vfStatus === 'hijack') link.hijackCount++
    link.status = ev.vfStatus
    link.lastSeq = ev.seq
    link.touchedAt = clock
    link.withdrawnAt = null
    if (link.rel === 'unknown') {
      const r = relForHop(ev, from, to)
      if (r !== 'unknown') link.rel = r
    }
  }

  if (ev.originAs !== 0) {
    const origin = g.nodes.get(ev.originAs)
    if (origin) origin.rpki = ev.rpkiStatus
  }
  if (ev.offenderAs !== 0 && (ev.vfStatus === 'leak' || ev.vfStatus === 'hijack')) {
    const off = g.nodes.get(ev.offenderAs)
    if (off) {
      off.offenderAt = clock
      off.offenderStatus = ev.vfStatus
    }
  }
  return structureChanged
}

// applyTopology seeds the graph from a full snapshot (names, prefix counts, edges),
// preserving any existing node positions.
export function applyTopology(g: GraphState, topo: Topology, clock: number): boolean {
  let changed = false
  for (const dto of topo.nodes) {
    if (dto.prefixCount > g.maxPrefixCount) g.maxPrefixCount = dto.prefixCount
  }
  for (const dto of topo.nodes) {
    const existing = g.nodes.get(dto.asn)
    if (!existing) {
      g.nodes.set(dto.asn, {
        asn: dto.asn,
        name: dto.name,
        prefixCount: dto.prefixCount,
        rpki: rpkiFromCounts(dto.rpki),
        offenderAt: 0,
        offenderStatus: 'valid',
        radius: radiusFor(dto.prefixCount, g.maxPrefixCount),
        bornAt: clock,
      })
      changed = true
    } else {
      if (dto.name) existing.name = dto.name
      existing.prefixCount = dto.prefixCount
      existing.rpki = rpkiFromCounts(dto.rpki)
      existing.radius = radiusFor(dto.prefixCount, g.maxPrefixCount)
    }
  }
  for (const dto of topo.edges) {
    const key = edgeKey(dto.from, dto.to)
    const existing = g.links.get(key)
    if (!existing) {
      g.links.set(key, {
        source: dto.from,
        target: dto.to,
        edgeKey: key,
        from: dto.from,
        to: dto.to,
        rel: dto.rel,
        status: dto.status,
        count: dto.count,
        leakCount: dto.leakCount,
        hijackCount: dto.hijackCount,
        lastSeq: 0,
        bornAt: clock,
        touchedAt: 0,
        withdrawnAt: null,
      })
      changed = true
    } else {
      existing.status = dto.status
      existing.rel = dto.rel
      existing.count = dto.count
      existing.leakCount = dto.leakCount
      existing.hijackCount = dto.hijackCount
    }
  }
  return changed
}

// applyStats refreshes prefix counts, RPKI status, and names for the most-active
// origin ASes from a periodic stats frame. It returns whether a new node was added.
export function applyStats(g: GraphState, stats: Stats, clock: number): boolean {
  let changed = false
  for (const o of stats.topOrigins) {
    if (o.prefixCount > g.maxPrefixCount) g.maxPrefixCount = o.prefixCount
  }
  for (const o of stats.topOrigins) {
    const existing = g.nodes.get(o.asn)
    if (!existing) {
      g.nodes.set(o.asn, {
        asn: o.asn,
        name: o.name,
        prefixCount: o.prefixCount,
        rpki: o.rpkiStatus,
        offenderAt: 0,
        offenderStatus: 'valid',
        radius: radiusFor(o.prefixCount, g.maxPrefixCount),
        bornAt: clock,
      })
      changed = true
    } else {
      if (o.name) existing.name = o.name
      existing.prefixCount = o.prefixCount
      existing.rpki = o.rpkiStatus
      existing.radius = radiusFor(o.prefixCount, g.maxPrefixCount)
    }
  }
  return changed
}
