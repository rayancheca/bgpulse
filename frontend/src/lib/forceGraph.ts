import {
  forceSimulation,
  forceLink,
  forceManyBody,
  forceX,
  forceY,
  forceCollide,
  type Simulation,
} from 'd3-force'

import type { SimNode, SimLink } from './types'

// Force tuning. forceX/forceY toward the viewport centre replace forceCenter so the
// constellation can drift but stays anchored without the rigid recentring jolt
// forceCenter causes when nodes are added.
export const FORCE = {
  CHARGE: -240,
  LINK_DISTANCE: 68,
  PEER_EXTRA: 28,
  LINK_STRENGTH: 0.35,
  COLLIDE_PAD: 4,
  CENTER_STRENGTH: 0.045,
  ALPHA_DECAY: 0.0228,
  ALPHA_TARGET_LIVE: 0.06, // keep the sim gently warm while events flow
  VELOCITY_DECAY: 0.4,
} as const

// buildSimulation wires the d3-force layout. It owns only the simulation math; the
// DOM/pixels are owned by React and the Canvas renderer.
export function buildSimulation(
  nodes: SimNode[],
  links: SimLink[],
  cx: number,
  cy: number,
): Simulation<SimNode, SimLink> {
  return forceSimulation<SimNode>(nodes)
    .force(
      'link',
      forceLink<SimNode, SimLink>(links)
        .id((d) => d.asn)
        .distance((l) => FORCE.LINK_DISTANCE + (l.rel === 'peer' ? FORCE.PEER_EXTRA : 0))
        .strength(FORCE.LINK_STRENGTH),
    )
    .force(
      'charge',
      forceManyBody<SimNode>().strength((d) => FORCE.CHARGE - d.radius * 2),
    )
    .force(
      'collide',
      forceCollide<SimNode>((d) => d.radius + FORCE.COLLIDE_PAD),
    )
    .force('x', forceX<SimNode>(cx).strength(FORCE.CENTER_STRENGTH))
    .force('y', forceY<SimNode>(cy).strength(FORCE.CENTER_STRENGTH))
    .velocityDecay(FORCE.VELOCITY_DECAY)
    .alphaDecay(FORCE.ALPHA_DECAY)
}
