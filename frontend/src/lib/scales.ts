import { NODE_RADIUS } from './constants'

// radiusFor maps an announced-prefix count to a node radius using a sqrt scale, so
// that node AREA is proportional to prefix count (perceptually correct for "how big
// is this AS").
export function radiusFor(prefixCount: number, maxPrefixCount: number): number {
  const max = Math.max(2, maxPrefixCount)
  const c = Math.max(1, Math.min(prefixCount, max))
  const t = (Math.sqrt(c) - 1) / (Math.sqrt(max) - 1) // 0..1
  return NODE_RADIUS.MIN + t * (NODE_RADIUS.MAX - NODE_RADIUS.MIN)
}

// edgeWidth maps a cumulative traversal count to a stroke width.
export function edgeWidth(count: number): number {
  return 1 + Math.min(2.5, Math.log10(1 + count))
}
