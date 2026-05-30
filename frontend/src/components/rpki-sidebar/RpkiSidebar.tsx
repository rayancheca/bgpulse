import { useStreamStore } from '../../hooks/useStreamStore'
import { COLOR } from '../../lib/constants'
import { formatAsn } from '../../lib/format'
import { RPKI_LABEL, RPKI_VAR } from '../../lib/status'
import type { OriginStat, RpkiStatus } from '../../lib/types'
import { Panel } from '../ui/Panel'
import { Sparkline } from './Sparkline'
import './rpki-sidebar.css'

// Sort Invalid origins first (the ones an operator cares about), then by prefix count.
const RANK: Record<RpkiStatus, number> = { invalid: 0, valid: 1, notfound: 2 }

export function RpkiSidebar() {
  const stats = useStreamStore((s) => s.stats)
  const origins = (stats?.topOrigins ?? []).slice().sort((a, b) => {
    if (a.rpkiStatus !== b.rpkiStatus) return RANK[a.rpkiStatus] - RANK[b.rpkiStatus]
    return b.prefixCount - a.prefixCount
  })

  return (
    <Panel className="rpki-sidebar" aria-label="RPKI origin validation by autonomous system">
      <div className="rpki-sidebar__head">
        <span className="panel-title">RPKI origins</span>
        <span className="label">{origins.length} active</span>
      </div>
      <ul className="rpki-sidebar__list">
        {origins.map((o) => (
          <RpkiCard key={o.asn} origin={o} />
        ))}
      </ul>
    </Panel>
  )
}

function RpkiCard({ origin }: { origin: OriginStat }) {
  const sparkColor = origin.rpkiStatus === 'invalid' ? RPKI_VAR.invalid : COLOR.accent
  return (
    <li className={`rpki-card rpki-card--${origin.rpkiStatus}`}>
      <div className="rpki-card__top">
        <span className="rpki-card__asn data">{formatAsn(origin.asn)}</span>
        <span className="rpki-card__stamp" style={{ color: RPKI_VAR[origin.rpkiStatus] }}>
          {RPKI_LABEL[origin.rpkiStatus]}
        </span>
      </div>
      <div className="rpki-card__meta">
        {origin.name ? <span className="rpki-card__name">{origin.name}</span> : <span />}
        <span className="meta">{origin.prefixCount} pfx</span>
      </div>
      <Sparkline data={origin.throughput} color={sparkColor} />
      <div className="rpki-card__counts data">
        <span style={{ color: RPKI_VAR.valid }}>{origin.valid}V</span>
        <span style={{ color: RPKI_VAR.invalid }}>{origin.invalid}I</span>
        <span style={{ color: RPKI_VAR.notfound }}>{origin.notfound}N</span>
      </div>
    </li>
  )
}
