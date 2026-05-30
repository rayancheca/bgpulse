import { describe, it, expect } from 'vitest'

import { zServerFrame, zClassifiedEvent } from './schema'
import { applyEvent, applyTopology, applyStats, createGraphState, edgeKey } from './graphStore'
import { radiusFor, edgeWidth } from './scales'
import { severity, rpkiFromCounts, worseRpki } from './status'
import { formatAsn, formatAsPath, formatTime, formatCommunity } from './format'
import type { ClassifiedEvent, Topology, Stats } from './types'

// A classified event matching the backend golden JSON (api/rest_test.go).
const goldenEvent = {
  id: 'synth-000042',
  seq: 42,
  timestamp: '2024-01-01T00:00:00Z',
  kind: 'announce',
  prefix: '10.0.0.0/24',
  peerAs: 65001,
  asPath: [65001, 3356, 174],
  nextHop: '192.0.2.1',
  communities: [{ asn: 2604, value: 100 }],
  originAs: 174,
  vfStatus: 'valid',
  rpkiStatus: 'valid',
  hops: [
    { from: 65001, to: 3356, rel: 'provider', isOffender: false },
    { from: 3356, to: 174, rel: 'provider', isOffender: false },
  ],
  offenderAs: 0,
  reason: '',
}

describe('schema', () => {
  it('validates the golden event frame', () => {
    const frame = { type: 'event', seq: 42, event: goldenEvent }
    const parsed = zServerFrame.safeParse(frame)
    expect(parsed.success).toBe(true)
  })

  it('rejects an event with a bad vfStatus', () => {
    const bad = { ...goldenEvent, vfStatus: 'totally-broken' }
    expect(zClassifiedEvent.safeParse(bad).success).toBe(false)
  })

  it('rejects an unknown frame type', () => {
    expect(zServerFrame.safeParse({ type: 'mystery' }).success).toBe(false)
  })
})

function leakEvent(): ClassifiedEvent {
  return {
    ...(goldenEvent as ClassifiedEvent),
    id: 'synth-000100',
    seq: 100,
    asPath: [1002, 2001, 1001],
    originAs: 1001,
    vfStatus: 'leak',
    offenderAs: 2001,
    hops: [],
  }
}

describe('graphStore', () => {
  it('builds nodes and directed edges from an announce', () => {
    const g = createGraphState()
    const changed = applyEvent(g, goldenEvent as ClassifiedEvent, 0)
    expect(changed).toBe(true)
    expect(g.nodes.size).toBe(3)
    expect(g.links.has(edgeKey(65001, 3356))).toBe(true)
    expect(g.links.has(edgeKey(3356, 174))).toBe(true)
    expect(g.links.get(edgeKey(65001, 3356))?.status).toBe('valid')
  })

  it('flags the offender node on a leak', () => {
    const g = createGraphState()
    applyEvent(g, leakEvent(), 1234)
    const off = g.nodes.get(2001)
    expect(off?.offenderAt).toBe(1234)
    expect(off?.offenderStatus).toBe('leak')
  })

  it('ignores withdraws for graph structure', () => {
    const g = createGraphState()
    const wd = { ...(goldenEvent as ClassifiedEvent), kind: 'withdraw' as const }
    expect(applyEvent(g, wd, 0)).toBe(false)
    expect(g.nodes.size).toBe(0)
  })

  it('seeds nodes and edges from a topology snapshot', () => {
    const g = createGraphState()
    const topo: Topology = {
      nodes: [{ asn: 174, name: 'Cogent', prefixCount: 5, rpki: { valid: 3, invalid: 0, notfound: 1 }, firstSeen: '', lastSeen: '' }],
      edges: [{ from: 174, to: 3356, status: 'valid', rel: 'peer', count: 2, leakCount: 0, hijackCount: 0, lastEvent: '', lastEventId: '' }],
      nodeCount: 1,
      edgeCount: 1,
      generated: '',
    }
    applyTopology(g, topo, 0)
    expect(g.nodes.get(174)?.name).toBe('Cogent')
    expect(g.nodes.get(174)?.rpki).toBe('valid')
    expect(g.links.has(edgeKey(174, 3356))).toBe(true)
  })

  it('refreshes top origins from a stats frame', () => {
    const g = createGraphState()
    const stats = {
      totalEvents: 0, announces: 0, withdraws: 0, leaks: 0, hijacks: 0,
      rpkiValid: 0, rpkiInvalid: 0, rpkiNotFound: 0, nodeCount: 0, edgeCount: 0, eventsPerSec: 0,
      topOrigins: [{ asn: 13335, name: 'Cloudflare', prefixCount: 9, rpkiStatus: 'invalid' as const, valid: 0, invalid: 2, notfound: 0, throughput: [] }],
    } satisfies Stats
    applyStats(g, stats, 0)
    expect(g.nodes.get(13335)?.prefixCount).toBe(9)
    expect(g.nodes.get(13335)?.rpki).toBe('invalid')
  })
})

describe('scales', () => {
  it('radiusFor is monotonic and clamped', () => {
    expect(radiusFor(1, 100)).toBeLessThan(radiusFor(50, 100))
    expect(radiusFor(50, 100)).toBeLessThan(radiusFor(100, 100))
    expect(radiusFor(200, 100)).toBe(radiusFor(100, 100)) // clamped
  })
  it('edgeWidth grows with count', () => {
    expect(edgeWidth(1)).toBeLessThan(edgeWidth(1000))
  })
})

describe('status', () => {
  it('severity orders hijack > leak > valid > unknown', () => {
    expect(severity('hijack')).toBeGreaterThan(severity('leak'))
    expect(severity('leak')).toBeGreaterThan(severity('valid'))
    expect(severity('valid')).toBeGreaterThan(severity('unknown'))
  })
  it('rpkiFromCounts and worseRpki prioritise invalid', () => {
    expect(rpkiFromCounts({ valid: 5, invalid: 1, notfound: 0 })).toBe('invalid')
    expect(rpkiFromCounts({ valid: 5, invalid: 0, notfound: 2 })).toBe('valid')
    expect(rpkiFromCounts({ valid: 0, invalid: 0, notfound: 2 })).toBe('notfound')
    expect(worseRpki('valid', 'invalid')).toBe('invalid')
    expect(worseRpki('notfound', 'valid')).toBe('valid')
  })
})

describe('format', () => {
  it('formats ASNs, paths, communities', () => {
    expect(formatAsn(174)).toBe('AS174')
    expect(formatAsPath([65001, 3356, 174])).toBe('65001 3356 174')
    expect(formatCommunity({ asn: 2604, value: 100 })).toBe('2604:100')
  })
  it('formats timestamps and handles bad input', () => {
    expect(formatTime('')).toBe('--:--:--')
    expect(formatTime('not-a-date')).toBe('--:--:--')
    expect(formatTime('2024-01-01T00:00:00Z')).toMatch(/^\d{2}:\d{2}:\d{2}$/)
  })
})
