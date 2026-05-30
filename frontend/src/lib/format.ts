import type { Community } from './types'

// formatAsn renders an ASN with the conventional "AS" prefix.
export function formatAsn(asn: number): string {
  return `AS${asn}`
}

// formatAsPath joins an AS_PATH into the space-separated form operators read.
export function formatAsPath(path: readonly number[]): string {
  return path.join(' ')
}

// formatCommunity renders a community as ASN:value.
export function formatCommunity(c: Community): string {
  return `${c.asn}:${c.value}`
}

// formatTime renders an RFC3339 timestamp as HH:MM:SS in local time, or "--:--:--"
// when absent/unparseable.
export function formatTime(iso: string): string {
  if (!iso) return '--:--:--'
  const t = Date.parse(iso)
  if (Number.isNaN(t)) return '--:--:--'
  const d = new Date(t)
  const p = (n: number) => String(n).padStart(2, '0')
  return `${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
}

// formatRate renders an events/second value compactly.
export function formatRate(rate: number): string {
  if (rate >= 100) return Math.round(rate).toString()
  return rate.toFixed(1)
}
