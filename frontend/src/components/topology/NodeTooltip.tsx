import { formatAsn } from '../../lib/format'
import { RPKI_LABEL, RPKI_VAR } from '../../lib/status'
import type { RpkiStatus } from '../../lib/types'

export interface HoverInfo {
  asn: number | null
  edge: string | null
  x: number
  y: number
  name: string
  prefixCount: number
  rpki: RpkiStatus
}

interface NodeTooltipProps {
  hover: HoverInfo
}

// NodeTooltip is a lightweight DOM overlay (not part of the canvas) shown for the
// hovered AS node.
export function NodeTooltip({ hover }: NodeTooltipProps) {
  if (hover.asn == null) return null
  return (
    <div className="node-tooltip" style={{ left: hover.x, top: hover.y }} role="tooltip">
      <div className="node-tooltip__asn data">{formatAsn(hover.asn)}</div>
      {hover.name ? <div className="node-tooltip__name">{hover.name}</div> : null}
      <div className="node-tooltip__row">
        <span className="label">prefixes</span>
        <span className="data">{hover.prefixCount}</span>
      </div>
      <div className="node-tooltip__row">
        <span className="label">rpki</span>
        <span className="node-tooltip__stamp" style={{ color: RPKI_VAR[hover.rpki] }}>
          {RPKI_LABEL[hover.rpki]}
        </span>
      </div>
    </div>
  )
}
