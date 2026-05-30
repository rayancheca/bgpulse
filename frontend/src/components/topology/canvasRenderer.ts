// Pure Canvas 2D renderer for the AS topology. Draws the whole graph in one pass per
// frame (no DOM churn): violet edges/nodes at rest, amber/crimson on anomaly, an
// offender ring that pulses then holds, RPKI rims on origins, and hover/selection
// emphasis. All colours are the raw hex from constants.ts (the canvas cannot read
// CSS custom properties).
import { COLOR, EDGE_FLASH_MS, OFFENDER_DECAY_MS, ROUTE_HEX, RPKI_HEX } from '../../lib/constants'
import { edgeWidth } from '../../lib/scales'
import type { SimLink, SimNode } from '../../lib/types'

const TAU = Math.PI * 2
const SPAWN_MS = 280

export interface RenderInput {
  ctx: CanvasRenderingContext2D
  nodes: SimNode[]
  links: SimLink[]
  width: number
  height: number
  clock: number
  reducedMotion: boolean
  selectedEdge: string | null
  hoveredAsn: number | null
  hoveredEdge: string | null
  focusedEdge: string | null
}

function endpoints(l: SimLink): [number, number, number, number] | null {
  const s = l.source
  const t = l.target
  if (typeof s === 'number' || typeof t === 'number') return null
  if (s.x == null || s.y == null || t.x == null || t.y == null) return null
  return [s.x, s.y, t.x, t.y]
}

function spawnRadius(n: SimNode, clock: number, reducedMotion: boolean): number {
  if (reducedMotion) return n.radius
  const age = clock - n.bornAt
  if (age >= SPAWN_MS) return n.radius
  const t = age / SPAWN_MS
  const eased = 1 - Math.pow(1 - t, 3) // ease-out cubic
  return n.radius * eased
}

export function draw(input: RenderInput): void {
  const { ctx, nodes, links, width, height, clock, reducedMotion, selectedEdge, hoveredAsn, hoveredEdge, focusedEdge } = input
  ctx.clearRect(0, 0, width, height)

  let selFrom = -1
  let selTo = -1
  if (selectedEdge) {
    const parts = selectedEdge.split('>')
    selFrom = Number(parts[0])
    selTo = Number(parts[1])
  }
  const dimOthers = selectedEdge != null

  drawEdges(ctx, links, clock, reducedMotion, { selectedEdge, hoveredEdge, focusedEdge, dimOthers, selFrom, selTo })
  drawNodes(ctx, nodes, clock, reducedMotion, { hoveredAsn, dimOthers, selFrom, selTo })
  ctx.globalAlpha = 1
}

interface EdgeCtx {
  selectedEdge: string | null
  hoveredEdge: string | null
  focusedEdge: string | null
  dimOthers: boolean
  selFrom: number
  selTo: number
}

function drawEdges(ctx: CanvasRenderingContext2D, links: SimLink[], clock: number, reducedMotion: boolean, c: EdgeCtx): void {
  ctx.lineCap = 'round'
  for (const l of links) {
    const e = endpoints(l)
    if (!e) continue
    const [x1, y1, x2, y2] = e
    const isSel = l.edgeKey === c.selectedEdge
    const emphasized = isSel || l.edgeKey === c.hoveredEdge || l.edgeKey === c.focusedEdge
    const incident = c.dimOthers && l.from === c.selFrom && l.to === c.selTo

    let alpha = l.status === 'valid' || l.status === 'unknown' ? 0.5 : 0.85
    if (c.dimOthers && !incident && !isSel) alpha = 0.12
    let w = edgeWidth(l.count)
    if (!reducedMotion && clock - l.touchedAt < EDGE_FLASH_MS) {
      const t = 1 - (clock - l.touchedAt) / EDGE_FLASH_MS
      alpha = Math.min(1, alpha + 0.4 * t)
      w += 1.2 * t
    }
    if (emphasized) {
      alpha = 1
      w += 1
    }

    const color = ROUTE_HEX[l.status]
    if (l.status === 'leak') {
      ctx.setLineDash([6, 5])
      ctx.lineDashOffset = reducedMotion ? 0 : -((clock / 40) % 11)
    } else {
      ctx.setLineDash([])
    }
    // soft glow for hijacks and the selected edge
    if (l.status === 'hijack' || isSel) {
      ctx.globalAlpha = alpha * 0.22
      ctx.strokeStyle = color
      ctx.lineWidth = w + 6
      ctx.beginPath()
      ctx.moveTo(x1, y1)
      ctx.lineTo(x2, y2)
      ctx.stroke()
    }
    ctx.globalAlpha = alpha
    ctx.strokeStyle = color
    ctx.lineWidth = w
    ctx.beginPath()
    ctx.moveTo(x1, y1)
    ctx.lineTo(x2, y2)
    ctx.stroke()
  }
  ctx.setLineDash([])
  ctx.globalAlpha = 1
}

interface NodeCtx {
  hoveredAsn: number | null
  dimOthers: boolean
  selFrom: number
  selTo: number
}

function drawNodes(ctx: CanvasRenderingContext2D, nodes: SimNode[], clock: number, reducedMotion: boolean, c: NodeCtx): void {
  for (const n of nodes) {
    if (n.x == null || n.y == null) continue
    const r = spawnRadius(n, clock, reducedMotion)
    const dimmed = c.dimOthers && n.asn !== c.selFrom && n.asn !== c.selTo

    // bloom
    ctx.globalAlpha = dimmed ? 0.05 : 0.1
    ctx.fillStyle = COLOR.accentGlow
    ctx.beginPath()
    ctx.arc(n.x, n.y, r + 6, 0, TAU)
    ctx.fill()

    // body
    ctx.globalAlpha = dimmed ? 0.3 : 1
    ctx.fillStyle = COLOR.nodeFill
    ctx.beginPath()
    ctx.arc(n.x, n.y, r, 0, TAU)
    ctx.fill()

    // RPKI rim on origins
    if (n.rpki === 'invalid') {
      ctx.globalAlpha = 1
      ctx.strokeStyle = RPKI_HEX.invalid
      ctx.lineWidth = 2
      ctx.beginPath()
      ctx.arc(n.x, n.y, r + 1.5, 0, TAU)
      ctx.stroke()
    } else if (n.rpki === 'valid' && !dimmed) {
      ctx.globalAlpha = 0.85
      ctx.strokeStyle = RPKI_HEX.valid
      ctx.lineWidth = 1.5
      ctx.beginPath()
      ctx.arc(n.x, n.y, r + 1.5, 0, TAU)
      ctx.stroke()
    }

    // offender ring (pulses then holds, decays out)
    const age = clock - n.offenderAt
    if (n.offenderAt > 0 && age < OFFENDER_DECAY_MS) {
      const pulse = reducedMotion ? 0.75 : 0.45 + 0.4 * Math.abs(Math.sin(clock / 300))
      ctx.globalAlpha = Math.min(1, pulse * (1 - age / OFFENDER_DECAY_MS) + 0.25)
      ctx.strokeStyle = ROUTE_HEX[n.offenderStatus]
      ctx.lineWidth = 2.5
      ctx.beginPath()
      ctx.arc(n.x, n.y, r + 7, 0, TAU)
      ctx.stroke()
    }

    // hover halo
    if (n.asn === c.hoveredAsn) {
      ctx.globalAlpha = 0.9
      ctx.strokeStyle = COLOR.accentGlow
      ctx.lineWidth = 2
      ctx.beginPath()
      ctx.arc(n.x, n.y, r + 4, 0, TAU)
      ctx.stroke()
    }

    // label prominent nodes and the hovered node
    if (r >= 12 || n.asn === c.hoveredAsn) {
      ctx.globalAlpha = n.asn === c.hoveredAsn ? 1 : 0.65
      ctx.fillStyle = COLOR.text
      ctx.font = `${n.asn === c.hoveredAsn ? 12 : 10}px "IBM Plex Mono", ui-monospace, monospace`
      ctx.textAlign = 'center'
      ctx.textBaseline = 'middle'
      ctx.fillText(String(n.asn), n.x, n.y - r - 8)
    }
  }
  ctx.globalAlpha = 1
}
