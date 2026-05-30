import { create } from 'zustand'

import {
  applyEvent,
  applyStats,
  applyTopology,
  createGraphState,
  type GraphState,
} from '../lib/graphStore'
import type { ClassifiedEvent, Stats, Topology } from '../lib/types'

function clock(): number {
  return typeof performance !== 'undefined' ? performance.now() : Date.now()
}

interface TopologyState {
  graph: GraphState
  // structureVersion increments only when a node/edge is added or removed, so the
  // canvas rebinds and reheats the simulation only when the graph SHAPE changes.
  structureVersion: number
  selectedEdge: string | null
  hoveredAsn: number | null
  ingestEvent: (ev: ClassifiedEvent) => void
  ingestTopology: (topo: Topology) => void
  ingestStats: (stats: Stats) => void
  selectEdge: (key: string | null) => void
  hoverAsn: (asn: number | null) => void
  reset: () => void
}

// useTopologyStore owns the live graph. The graph Maps are mutated in place (so
// d3-force keeps node positions); the canvas reads them every frame via getState(),
// while React chrome subscribes to the small primitives below.
export const useTopologyStore = create<TopologyState>()((set, get) => ({
  graph: createGraphState(),
  structureVersion: 0,
  selectedEdge: null,
  hoveredAsn: null,
  ingestEvent: (ev) => {
    if (applyEvent(get().graph, ev, clock())) {
      set((s) => ({ structureVersion: s.structureVersion + 1 }))
    }
  },
  ingestTopology: (topo) => {
    if (applyTopology(get().graph, topo, clock())) {
      set((s) => ({ structureVersion: s.structureVersion + 1 }))
    }
  },
  ingestStats: (stats) => {
    if (applyStats(get().graph, stats, clock())) {
      set((s) => ({ structureVersion: s.structureVersion + 1 }))
    }
  },
  selectEdge: (selectedEdge) => set({ selectedEdge }),
  hoverAsn: (hoveredAsn) => set({ hoveredAsn }),
  reset: () =>
    set((s) => ({
      graph: createGraphState(),
      structureVersion: s.structureVersion + 1,
      selectedEdge: null,
      hoveredAsn: null,
    })),
}))
