// Wire types are inferred from the zod schemas (see schema.ts) and re-exported here
// so the rest of the app imports from one place. This file also defines the
// frontend-only simulation types that d3-force operates on.
export type {
  VfStatus,
  RpkiStatus,
  Relationship,
  Kind,
  Community,
  RpkiCounts,
  PathHop,
  ClassifiedEvent,
  Node,
  Edge,
  Topology,
  OriginStat,
  Stats,
  Snapshot,
  ServerFrame,
} from './schema'

import type { Relationship, VfStatus, RpkiStatus } from './schema'

// SimNode is a graph vertex in the force simulation. d3-force mutates x/y/vx/vy in
// place (the one sanctioned mutation, isolated to the simulation layer).
export interface SimNode {
  asn: number
  name: string
  prefixCount: number
  rpki: RpkiStatus
  offenderAt: number // sim-clock ms when this node was last an anomaly offender (0 = never)
  offenderStatus: VfStatus // the anomaly kind for the offender ring (leak/hijack)
  radius: number
  bornAt: number
  x?: number
  y?: number
  vx?: number
  vy?: number
  fx?: number | null
  fy?: number | null
}

// SimLink is a directed edge in the force simulation. d3-force replaces the numeric
// source/target with SimNode references after binding.
export interface SimLink {
  source: number | SimNode
  target: number | SimNode
  edgeKey: string
  from: number
  to: number
  rel: Relationship
  status: VfStatus
  count: number // cumulative traversals (drives edge width)
  leakCount: number
  hijackCount: number
  lastSeq: number
  bornAt: number
  touchedAt: number // sim-clock ms of the most recent traversing event (for flash)
  withdrawnAt: number | null
}
