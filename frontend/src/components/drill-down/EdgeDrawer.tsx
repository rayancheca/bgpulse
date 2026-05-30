import { useEffect } from 'react'

import { useStreamStore } from '../../hooks/useStreamStore'
import { useTopologyStore } from '../../hooks/useTopologyStore'
import { formatAsn, formatTime } from '../../lib/format'
import { REL_LABEL, RPKI_LABEL, RPKI_VAR, VF_LABEL, VF_VAR } from '../../lib/status'
import type { VfStatus } from '../../lib/types'
import { AsPath } from '../event-stream/AsPath'
import './drill-down.css'

function pathHasEdge(path: number[], from: number, to: number): boolean {
  for (let i = 0; i + 1 < path.length; i++) {
    if (path[i] === from && path[i + 1] === to) return true
  }
  return false
}

// EdgeDrawer slides in when an edge is selected and shows the adjacency's verdict
// plus the raw UPDATEs (from the live buffer) that produced it.
export function EdgeDrawer() {
  const selectedEdge = useTopologyStore((s) => s.selectedEdge)
  const selectEdge = useTopologyStore((s) => s.selectEdge)
  const events = useStreamStore((s) => s.events)

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') selectEdge(null)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [selectEdge])

  if (!selectedEdge) return null
  const parts = selectedEdge.split('>')
  const from = Number(parts[0])
  const to = Number(parts[1])
  const link = useTopologyStore.getState().graph.links.get(selectedEdge)
  const status: VfStatus = link?.status ?? 'unknown'
  const edgeEvents = events.filter((ev) => pathHasEdge(ev.asPath, from, to)).slice(0, 40)

  return (
    <aside className="drawer" role="dialog" aria-label={`Adjacency AS${from} to AS${to}`}>
      <header className="drawer__head">
        <div>
          <div className="label">AS adjacency</div>
          <div className="drawer__pair data">
            {formatAsn(from)} <span className="drawer__arrow">→</span> {formatAsn(to)}
          </div>
        </div>
        <button className="drawer__close" onClick={() => selectEdge(null)} aria-label="Close drawer">
          ×
        </button>
      </header>

      <div className="drawer__verdict" style={{ borderColor: VF_VAR[status] }}>
        <span className="drawer__verdict-status" style={{ color: VF_VAR[status] }}>
          {VF_LABEL[status]}
        </span>
        <div className="meta drawer__verdict-meta">
          relationship {link ? REL_LABEL[link.rel] : '—'} · {link?.count ?? 0} updates · {link?.leakCount ?? 0} leaks ·{' '}
          {link?.hijackCount ?? 0} hijacks
        </div>
      </div>

      <div className="drawer__events">
        <div className="label drawer__events-head">UPDATEs producing this edge · {edgeEvents.length}</div>
        <ul className="drawer__list">
          {edgeEvents.map((ev) => (
            <li key={ev.id} className="drawer__event">
              <div className="drawer__event-top">
                <span className="meta">{formatTime(ev.timestamp)}</span>
                <span className="data drawer__event-prefix">{ev.prefix}</span>
                <span className="data" style={{ color: RPKI_VAR[ev.rpkiStatus] }}>
                  {RPKI_LABEL[ev.rpkiStatus]}
                </span>
              </div>
              <AsPath ev={ev} />
              {ev.reason ? <div className="drawer__reason meta">{ev.reason}</div> : null}
            </li>
          ))}
          {edgeEvents.length === 0 ? (
            <li className="meta drawer__empty">No recent UPDATEs for this adjacency in the live buffer.</li>
          ) : null}
        </ul>
      </div>
    </aside>
  )
}
