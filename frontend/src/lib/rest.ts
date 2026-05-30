// Minimal REST helper that unwraps the backend response envelope { ok, data, error }.

export interface Health {
  ok: boolean
  mode: string
  version: string
  uptimeSec: number
  sources: { bgp: string; relationships: string; rpki: string; liveFellBack: boolean }
}

export async function getJSON<T>(path: string): Promise<T> {
  const res = await fetch(path)
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
  const env = (await res.json()) as { ok: boolean; data?: T; error?: string }
  if (!env.ok || env.data === undefined) {
    throw new Error(env.error ?? 'request failed')
  }
  return env.data
}

export function getHealth(): Promise<Health> {
  return getJSON<Health>('/api/health')
}
