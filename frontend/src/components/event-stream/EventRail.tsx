import type { CSSProperties } from 'react'

import { useStreamStore } from '../../hooks/useStreamStore'
import { formatTime } from '../../lib/format'
import { RPKI_LABEL, RPKI_VAR, VF_LABEL, VF_VAR } from '../../lib/status'
import type { ClassifiedEvent } from '../../lib/types'
import { Panel } from '../ui/Panel'
import { AsPath } from './AsPath'
import './event-stream.css'

export function EventRail() {
  const events = useStreamStore((s) => s.events)
  return (
    <Panel className="event-rail" aria-label="Live BGP update stream">
      <div className="event-rail__head">
        <span className="panel-title">UPDATE stream</span>
        <span className="label">{events.length} recent</span>
      </div>
      <ul className="event-rail__list">
        {events.map((ev) => (
          <EventRow key={ev.id} ev={ev} />
        ))}
      </ul>
    </Panel>
  )
}

function EventRow({ ev }: { ev: ClassifiedEvent }) {
  const isWithdraw = ev.kind === 'withdraw'
  const anomaly = ev.vfStatus === 'leak' || ev.vfStatus === 'hijack'
  const rowStyle = anomaly ? ({ '--row-accent': VF_VAR[ev.vfStatus] } as CSSProperties) : undefined
  return (
    <li className={`event-row${anomaly ? ' event-row--anomaly' : ''}`} style={rowStyle}>
      <span className="event-row__time meta">{formatTime(ev.timestamp)}</span>
      <span className={`event-row__kind event-row__kind--${ev.kind}`}>{isWithdraw ? 'W' : 'A'}</span>
      <span className="event-row__prefix data">{ev.prefix}</span>
      {isWithdraw ? <span className="event-row__withdraw label">withdrawn</span> : <AsPath ev={ev} />}
      {!isWithdraw ? (
        <span className="event-row__tags">
          {anomaly ? (
            <span className="event-row__tag" style={{ color: VF_VAR[ev.vfStatus] }}>
              {VF_LABEL[ev.vfStatus]}
            </span>
          ) : null}
          <span className="event-row__rpki" style={{ color: RPKI_VAR[ev.rpkiStatus] }}>
            {RPKI_LABEL[ev.rpkiStatus]}
          </span>
        </span>
      ) : null}
    </li>
  )
}
