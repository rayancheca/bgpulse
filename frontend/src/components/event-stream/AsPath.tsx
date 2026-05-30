import { Fragment } from 'react'

import type { ClassifiedEvent } from '../../lib/types'

// AsPath renders an AS_PATH as mono hops, highlighting the offending hop on a leak
// or hijack so the violation is legible at a glance.
export function AsPath({ ev }: { ev: ClassifiedEvent }) {
  return (
    <span className="aspath data">
      {ev.asPath.map((asn, i) => (
        <Fragment key={i}>
          {i > 0 ? <span className="aspath__sep"> </span> : null}
          <span
            className={`aspath__hop${asn === ev.offenderAs && ev.offenderAs !== 0 ? ' aspath__hop--offender' : ''}`}
          >
            {asn}
          </span>
        </Fragment>
      ))}
    </span>
  )
}
