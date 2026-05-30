// Zod schemas mirroring the backend JSON wire contract (backend/internal/api/dto.go,
// pinned by the golden test there). Every inbound WebSocket frame is validated
// against these at the boundary; the wire TypeScript types are inferred from them so
// the schema and the types can never drift.
import { z } from 'zod'

export const zVfStatus = z.enum(['valid', 'leak', 'hijack', 'unknown'])
export const zRpkiStatus = z.enum(['valid', 'invalid', 'notfound'])
export const zRelationship = z.enum(['customer', 'provider', 'peer', 'sibling', 'unknown'])
export const zKind = z.enum(['announce', 'withdraw'])

export const zCommunity = z.object({ asn: z.number(), value: z.number() })

export const zPathHop = z.object({
  from: z.number(),
  to: z.number(),
  rel: zRelationship,
  isOffender: z.boolean(),
})

export const zClassifiedEvent = z.object({
  id: z.string(),
  seq: z.number(),
  timestamp: z.string(),
  kind: zKind,
  prefix: z.string(),
  peerAs: z.number(),
  asPath: z.array(z.number()),
  nextHop: z.string(),
  communities: z.array(zCommunity),
  originAs: z.number(),
  vfStatus: zVfStatus,
  rpkiStatus: zRpkiStatus,
  hops: z.array(zPathHop),
  offenderAs: z.number(),
  reason: z.string(),
})

export const zRpkiCounts = z.object({
  valid: z.number(),
  invalid: z.number(),
  notfound: z.number(),
})

export const zNode = z.object({
  asn: z.number(),
  name: z.string(),
  prefixCount: z.number(),
  rpki: zRpkiCounts,
  firstSeen: z.string(),
  lastSeen: z.string(),
})

export const zEdge = z.object({
  from: z.number(),
  to: z.number(),
  status: zVfStatus,
  rel: zRelationship,
  count: z.number(),
  leakCount: z.number(),
  hijackCount: z.number(),
  lastEvent: z.string(),
  lastEventId: z.string(),
})

export const zTopology = z.object({
  nodes: z.array(zNode),
  edges: z.array(zEdge),
  nodeCount: z.number(),
  edgeCount: z.number(),
  generated: z.string(),
})

export const zOriginStat = z.object({
  asn: z.number(),
  name: z.string(),
  prefixCount: z.number(),
  rpkiStatus: zRpkiStatus,
  valid: z.number(),
  invalid: z.number(),
  notfound: z.number(),
  throughput: z.array(z.number()),
})

export const zStats = z.object({
  totalEvents: z.number(),
  announces: z.number(),
  withdraws: z.number(),
  leaks: z.number(),
  hijacks: z.number(),
  rpkiValid: z.number(),
  rpkiInvalid: z.number(),
  rpkiNotFound: z.number(),
  nodeCount: z.number(),
  edgeCount: z.number(),
  eventsPerSec: z.number(),
  topOrigins: z.array(zOriginStat),
})

export const zSnapshot = z.object({
  topology: zTopology,
  events: z.array(zClassifiedEvent),
  stats: zStats,
})

export const zServerFrame = z.discriminatedUnion('type', [
  z.object({ type: z.literal('snapshot'), seq: z.number().optional(), snapshot: zSnapshot }),
  z.object({ type: z.literal('event'), seq: z.number(), event: zClassifiedEvent }),
  z.object({ type: z.literal('stats'), seq: z.number(), stats: zStats }),
])

// REST envelope: { ok, data?, error? }.
export function zEnvelope<T extends z.ZodTypeAny>(data: T) {
  return z.object({ ok: z.boolean(), data: data.optional(), error: z.string().optional() })
}

// ---- Wire types inferred from the schemas (single source of truth) ----
export type VfStatus = z.infer<typeof zVfStatus>
export type RpkiStatus = z.infer<typeof zRpkiStatus>
export type Relationship = z.infer<typeof zRelationship>
export type Kind = z.infer<typeof zKind>
export type Community = z.infer<typeof zCommunity>
export type RpkiCounts = z.infer<typeof zRpkiCounts>
export type PathHop = z.infer<typeof zPathHop>
export type ClassifiedEvent = z.infer<typeof zClassifiedEvent>
export type Node = z.infer<typeof zNode>
export type Edge = z.infer<typeof zEdge>
export type Topology = z.infer<typeof zTopology>
export type OriginStat = z.infer<typeof zOriginStat>
export type Stats = z.infer<typeof zStats>
export type Snapshot = z.infer<typeof zSnapshot>
export type ServerFrame = z.infer<typeof zServerFrame>
