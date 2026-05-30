import type { VfStatus, RpkiStatus, Relationship, RpkiCounts } from './types'

// CSS custom-property references for DOM styling (Canvas uses constants.ts hex).
export const VF_VAR: Record<VfStatus, string> = {
  valid: 'var(--color-route-normal)',
  leak: 'var(--color-route-leak)',
  hijack: 'var(--color-route-hijack)',
  unknown: 'var(--color-route-unknown)',
}

export const RPKI_VAR: Record<RpkiStatus, string> = {
  valid: 'var(--color-rpki-valid)',
  invalid: 'var(--color-rpki-invalid)',
  notfound: 'var(--color-rpki-notfound)',
}

// Display labels (the wire tokens are lowercase; the UI uses the RTR vocabulary).
export const RPKI_LABEL: Record<RpkiStatus, string> = {
  valid: 'Valid',
  invalid: 'Invalid',
  notfound: 'NotFound',
}

export const VF_LABEL: Record<VfStatus, string> = {
  valid: 'Normal',
  leak: 'Leak',
  hijack: 'Hijack',
  unknown: 'Unknown',
}

export const REL_LABEL: Record<Relationship, string> = {
  customer: 'customer',
  provider: 'provider',
  peer: 'peer',
  sibling: 'sibling',
  unknown: 'unknown',
}

// severity ranks statuses: hijack > leak > valid > unknown.
export function severity(s: VfStatus): number {
  switch (s) {
    case 'hijack':
      return 3
    case 'leak':
      return 2
    case 'valid':
      return 1
    default:
      return 0
  }
}

// rpkiFromCounts derives a representative status: Invalid if any invalid, else Valid
// if any valid, else NotFound — matching the backend's worst-status logic.
export function rpkiFromCounts(c: RpkiCounts): RpkiStatus {
  if (c.invalid > 0) return 'invalid'
  if (c.valid > 0) return 'valid'
  return 'notfound'
}

// worseRpki returns the more alarming of two statuses (Invalid > Valid > NotFound).
export function worseRpki(a: RpkiStatus, b: RpkiStatus): RpkiStatus {
  if (a === 'invalid' || b === 'invalid') return 'invalid'
  if (a === 'valid' || b === 'valid') return 'valid'
  return 'notfound'
}
