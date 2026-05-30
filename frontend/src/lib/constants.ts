import type { VfStatus, RpkiStatus } from './types'

// Raw hex colours for the Canvas 2D context (which cannot read CSS custom
// properties). These MUST stay in sync with styles/tokens.css.
export const ROUTE_HEX: Record<VfStatus, string> = {
  valid: '#7c6cf0',
  leak: '#f2a23c',
  hijack: '#f2415a',
  unknown: '#4a4458',
}

export const RPKI_HEX: Record<RpkiStatus, string> = {
  valid: '#34d399',
  invalid: '#f2415a',
  notfound: '#8a8198',
}

export const COLOR = {
  bg: '#08070d',
  accent: '#8b5cf6',
  accentGlow: '#c4b5fd',
  withdraw: '#645c78',
  text: '#ece8f5',
  textDim: '#9b93b0',
  border: '#272234',
  nodeFill: '#7c6cf0',
} as const

export const MAX_EVENTS = 500 // bounded event ring in the stream store
export const SPARK_SAMPLES = 30 // matches backend sparkBuckets
export const NODE_RADIUS = { MIN: 4, MAX: 32 } as const

// Animation windows (ms of sim-clock).
export const OFFENDER_DECAY_MS = 6000 // how long an offender ring pulses/holds
export const EDGE_FLASH_MS = 900 // how long an edge stays brightened after a hit

// Default WebSocket URL: same-origin /ws (works through the dev proxy and nginx).
export const WS_URL: string =
  (import.meta.env.VITE_WS_URL as string | undefined) ??
  `${typeof location !== 'undefined' && location.protocol === 'https:' ? 'wss' : 'ws'}://${
    typeof location !== 'undefined' ? location.host : 'localhost:8080'
  }/ws`
