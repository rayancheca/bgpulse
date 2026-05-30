import { useEffect, useRef, useState } from 'react'
import type { ForceLink, ForceX, ForceY, Simulation } from 'd3-force'

import { useReducedMotion } from '../../hooks/useReducedMotion'
import { useResizeObserver } from '../../hooks/useResizeObserver'
import { useTopologyStore } from '../../hooks/useTopologyStore'
import { buildSimulation, FORCE } from '../../lib/forceGraph'
import type { SimLink, SimNode } from '../../lib/types'
import { draw } from './canvasRenderer'
import { edgeAt, nodeAt } from './hitTest'
import { NodeTooltip, type HoverInfo } from './NodeTooltip'
import './topology.css'

const NO_HOVER: HoverInfo = { asn: null, edge: null, x: 0, y: 0, name: '', prefixCount: 0, rpki: 'notfound' }

export function TopologyCanvas() {
  const { ref: containerRef, size } = useResizeObserver<HTMLDivElement>()
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const simRef = useRef<Simulation<SimNode, SimLink> | null>(null)
  const nodesRef = useRef<SimNode[]>([])
  const linksRef = useRef<SimLink[]>([])
  const focusedEdgeRef = useRef<string | null>(null)

  const reduced = useReducedMotion()
  const reducedRef = useRef(reduced)
  reducedRef.current = reduced

  const structureVersion = useTopologyStore((s) => s.structureVersion)
  const selectEdge = useTopologyStore((s) => s.selectEdge)

  const [hover, setHover] = useState<HoverInfo>(NO_HOVER)
  const hoverRef = useRef(hover)
  hoverRef.current = hover

  // Size the canvas for the device pixel ratio and recentre the forces on resize.
  useEffect(() => {
    const c = canvasRef.current
    if (!c || size.width === 0) return
    c.width = Math.round(size.width * size.dpr)
    c.height = Math.round(size.height * size.dpr)
    c.style.width = `${size.width}px`
    c.style.height = `${size.height}px`
    const sim = simRef.current
    if (sim) {
      ;(sim.force('x') as ForceX<SimNode> | undefined)?.x(size.width / 2)
      ;(sim.force('y') as ForceY<SimNode> | undefined)?.y(size.height / 2)
      sim.alpha(Math.max(sim.alpha(), 0.25))
    }
  }, [size.width, size.height, size.dpr])

  // (Re)bind the simulation when the graph structure changes.
  useEffect(() => {
    const g = useTopologyStore.getState().graph
    nodesRef.current = Array.from(g.nodes.values())
    linksRef.current = Array.from(g.links.values())
    const cx = size.width ? size.width / 2 : 480
    const cy = size.height ? size.height / 2 : 320
    if (!simRef.current) {
      const sim = buildSimulation(nodesRef.current, linksRef.current, cx, cy)
      sim.stop() // ticked manually in the render loop
      sim.alphaTarget(reducedRef.current ? 0 : FORCE.ALPHA_TARGET_LIVE)
      simRef.current = sim
    } else {
      const sim = simRef.current
      sim.nodes(nodesRef.current)
      ;(sim.force('link') as ForceLink<SimNode, SimLink> | undefined)?.links(linksRef.current)
      sim.alphaTarget(reducedRef.current ? 0 : FORCE.ALPHA_TARGET_LIVE)
      sim.alpha(Math.max(sim.alpha(), 0.35))
    }
  }, [structureVersion, size.width, size.height])

  // The render loop: tick the simulation (manually) and draw every frame.
  useEffect(() => {
    let raf = 0
    const loop = () => {
      const c = canvasRef.current
      const sim = simRef.current
      const ctx = c?.getContext('2d')
      if (c && sim && ctx && size.width > 0) {
        // alphaTarget keeps the sim gently warm while live (0 under reduced motion,
        // so it converges and stops); tick until it cools below alphaMin.
        if (sim.alpha() > sim.alphaMin()) sim.tick()
        ctx.save()
        ctx.scale(size.dpr, size.dpr)
        draw({
          ctx,
          nodes: nodesRef.current,
          links: linksRef.current,
          width: size.width,
          height: size.height,
          clock: performance.now(),
          reducedMotion: reducedRef.current,
          selectedEdge: useTopologyStore.getState().selectedEdge,
          hoveredAsn: hoverRef.current.asn,
          hoveredEdge: hoverRef.current.edge,
          focusedEdge: focusedEdgeRef.current,
        })
        ctx.restore()
      }
      raf = requestAnimationFrame(loop)
    }
    raf = requestAnimationFrame(loop)
    return () => cancelAnimationFrame(raf)
  }, [size.width, size.height, size.dpr])

  function pointerXY(e: React.PointerEvent | React.MouseEvent): [number, number] {
    const c = canvasRef.current
    if (!c) return [0, 0]
    const rect = c.getBoundingClientRect()
    return [e.clientX - rect.left, e.clientY - rect.top]
  }

  function onMove(e: React.PointerEvent) {
    const [x, y] = pointerXY(e)
    const asn = nodeAt(nodesRef.current, x, y)
    const edge = asn == null ? edgeAt(linksRef.current, x, y) : null
    if (asn != null) {
      const n = useTopologyStore.getState().graph.nodes.get(asn)
      setHover({ asn, edge: null, x, y, name: n?.name ?? '', prefixCount: n?.prefixCount ?? 0, rpki: n?.rpki ?? 'notfound' })
    } else {
      setHover({ asn: null, edge, x, y, name: '', prefixCount: 0, rpki: 'notfound' })
    }
    const c = canvasRef.current
    if (c) c.style.cursor = asn != null || edge != null ? 'pointer' : 'default'
  }

  function onClick(e: React.MouseEvent) {
    const [x, y] = pointerXY(e)
    const edge = edgeAt(linksRef.current, x, y)
    if (edge) {
      selectEdge(edge)
      return
    }
    const asn = nodeAt(nodesRef.current, x, y)
    if (asn != null) {
      const n = useTopologyStore.getState().graph.nodes.get(asn)
      if (n) {
        const pinned = n.fx != null
        n.fx = pinned ? null : n.x
        n.fy = pinned ? null : n.y
        simRef.current?.alpha(0.3)
      }
      return
    }
    selectEdge(null)
  }

  function onKeyDown(e: React.KeyboardEvent) {
    const links = linksRef.current
    if (links.length === 0) return
    if (e.key === 'ArrowRight' || e.key === 'ArrowLeft') {
      e.preventDefault()
      const keys = links.map((l) => l.edgeKey)
      const cur = focusedEdgeRef.current
      let idx = cur ? keys.indexOf(cur) : -1
      idx = (idx + (e.key === 'ArrowRight' ? 1 : -1) + keys.length) % keys.length
      focusedEdgeRef.current = keys[idx] ?? null
    } else if (e.key === 'Enter' && focusedEdgeRef.current) {
      selectEdge(focusedEdgeRef.current)
    } else if (e.key === 'Escape') {
      selectEdge(null)
    }
  }

  return (
    <div className="topology" ref={containerRef}>
      <canvas
        ref={canvasRef}
        className="topology__canvas"
        role="application"
        tabIndex={0}
        aria-label="Autonomous System topology. Use arrow keys to move between edges and Enter to inspect one."
        onPointerMove={onMove}
        onPointerLeave={() => setHover(NO_HOVER)}
        onClick={onClick}
        onKeyDown={onKeyDown}
      />
      <NodeTooltip hover={hover} />
    </div>
  )
}
