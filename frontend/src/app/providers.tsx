import type { ReactNode } from 'react'

import { useWebSocket } from '../hooks/useWebSocket'

// Providers establishes the live WebSocket connection for the app tree.
export function Providers({ children }: { children: ReactNode }) {
  useWebSocket()
  return <>{children}</>
}
