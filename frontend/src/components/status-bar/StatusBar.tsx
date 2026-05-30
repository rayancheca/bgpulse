import { useEffect, useRef, useState } from 'react'

import { useStreamStore, type Connection } from '../../hooks/useStreamStore'
import { formatRate } from '../../lib/format'
import { getHealth } from '../../lib/rest'
import { StatusDot } from '../ui/StatusDot'
import './status-bar.css'

const CONN_COLOR: Record<Connection, string> = {
  connecting: 'var(--color-route-leak)',
  open: 'var(--color-rpki-valid)',
  closed: 'var(--color-route-hijack)',
}
const CONN_LABEL: Record<Connection, string> = {
  connecting: 'connecting',
  open: 'live',
  closed: 'reconnecting',
}

export function StatusBar() {
  const connection = useStreamStore((s) => s.connection)
  const stats = useStreamStore((s) => s.stats)
  const [mode, setMode] = useState('demo')

  useEffect(() => {
    getHealth()
      .then((h) => setMode(h.mode))
      .catch(() => {})
  }, [])

  return (
    <header className="status-bar panel" aria-label="Status">
      <div className="status-bar__group">
        <span className="status-bar__brand">BGPulse</span>
        <span className="status-bar__mode">{mode.toUpperCase()}</span>
        <StatusDot color={CONN_COLOR[connection]} pulse={connection !== 'open'} title={CONN_LABEL[connection]} />
        <span className="label">{CONN_LABEL[connection]}</span>
      </div>
      <div className="status-bar__group status-bar__center">
        <Metric label="events/s" value={formatRate(stats?.eventsPerSec ?? 0)} />
        <Metric label="total" value={String(stats?.totalEvents ?? 0)} />
        <Metric label="ASNs" value={String(stats?.nodeCount ?? 0)} />
        <Metric label="paths" value={String(stats?.edgeCount ?? 0)} />
      </div>
      <div className="status-bar__group">
        <Counter label="leaks" value={stats?.leaks ?? 0} color="var(--color-route-leak)" />
        <Counter label="hijacks" value={stats?.hijacks ?? 0} color="var(--color-route-hijack)" />
      </div>
    </header>
  )
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="status-bar__metric">
      <span className="data status-bar__metric-val">{value}</span>
      <span className="label">{label}</span>
    </div>
  )
}

function Counter({ label, value, color }: { label: string; value: number; color: string }) {
  const prev = useRef(value)
  const [flash, setFlash] = useState(false)
  useEffect(() => {
    if (value > prev.current) {
      setFlash(true)
      const t = setTimeout(() => setFlash(false), 450)
      prev.current = value
      return () => clearTimeout(t)
    }
    prev.current = value
    return undefined
  }, [value])
  return (
    <div className={`status-bar__counter${flash ? ' status-bar__counter--flash' : ''}`} style={{ color }}>
      <span className="counter">{value}</span>
      <span className="label status-bar__counter-label">{label}</span>
    </div>
  )
}
