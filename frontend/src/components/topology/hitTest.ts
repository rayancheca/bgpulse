// Hit testing against simulation positions (resolution-independent, no DOM events on
// shapes). Linear scans are fine at the few-hundred-node scale of this tool.
import type { SimLink, SimNode } from '../../lib/types'

const EDGE_TOLERANCE = 6 // px

// nodeAt returns the ASN of the node under (x, y), or null.
export function nodeAt(nodes: SimNode[], x: number, y: number): number | null {
  let best: number | null = null
  let bestDist = Infinity
  for (const n of nodes) {
    if (n.x == null || n.y == null) continue
    const dx = x - n.x
    const dy = y - n.y
    const d = Math.hypot(dx, dy)
    if (d <= n.radius + 3 && d < bestDist) {
      bestDist = d
      best = n.asn
    }
  }
  return best
}

// edgeAt returns the edgeKey of the edge under (x, y), or null, using point-to-
// segment distance.
export function edgeAt(links: SimLink[], x: number, y: number): string | null {
  let best: string | null = null
  let bestDist = EDGE_TOLERANCE
  for (const l of links) {
    const s = l.source
    const t = l.target
    if (typeof s === 'number' || typeof t === 'number') continue
    if (s.x == null || s.y == null || t.x == null || t.y == null) continue
    const d = pointToSegment(x, y, s.x, s.y, t.x, t.y)
    if (d < bestDist) {
      bestDist = d
      best = l.edgeKey
    }
  }
  return best
}

function pointToSegment(px: number, py: number, x1: number, y1: number, x2: number, y2: number): number {
  const dx = x2 - x1
  const dy = y2 - y1
  const lenSq = dx * dx + dy * dy
  if (lenSq === 0) return Math.hypot(px - x1, py - y1)
  let t = ((px - x1) * dx + (py - y1) * dy) / lenSq
  t = Math.max(0, Math.min(1, t))
  return Math.hypot(px - (x1 + t * dx), py - (y1 + t * dy))
}
