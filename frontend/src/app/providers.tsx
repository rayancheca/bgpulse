import { useEffect, type ReactNode } from 'react'

import { useStreamStore } from '../hooks/useStreamStore'
import { useTopologyStore } from '../hooks/useTopologyStore'
import { useWebSocket } from '../hooks/useWebSocket'

// Providers establishes the live WebSocket connection for the app tree and exposes a
// read-only debug handle (used by the E2E/screenshot tooling and for live inspection).
export function Providers({ children }: { children: ReactNode }) {
  useWebSocket()
  useEffect(() => {
    ;(window as unknown as { __bgpulse?: unknown }).__bgpulse = {
      topology: useTopologyStore,
      stream: useStreamStore,
    }
  }, [])
  return <>{children}</>
}
