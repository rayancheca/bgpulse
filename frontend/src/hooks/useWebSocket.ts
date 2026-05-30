import { useEffect } from 'react'

import { WS_URL } from '../lib/constants'
import { zServerFrame } from '../lib/schema'
import { useStreamStore } from './useStreamStore'
import { useTopologyStore } from './useTopologyStore'

const BACKOFF_MIN = 250
const BACKOFF_MAX = 8000

// dispatchFrame validates one inbound text frame and routes it to the stores. A
// malformed frame is dropped, never crashing the graph (boundary validation).
function dispatchFrame(data: unknown): void {
  if (typeof data !== 'string') return
  let json: unknown
  try {
    json = JSON.parse(data)
  } catch {
    return
  }
  const parsed = zServerFrame.safeParse(json)
  if (!parsed.success) return

  const frame = parsed.data
  const topo = useTopologyStore.getState()
  const stream = useStreamStore.getState()
  switch (frame.type) {
    case 'snapshot':
      topo.reset()
      topo.ingestTopology(frame.snapshot.topology)
      topo.ingestStats(frame.snapshot.stats)
      stream.setEvents(frame.snapshot.events)
      stream.setStats(frame.snapshot.stats)
      break
    case 'event':
      topo.ingestEvent(frame.event)
      stream.pushEvent(frame.event)
      break
    case 'stats':
      topo.ingestStats(frame.stats)
      stream.setStats(frame.stats)
      break
  }
}

// useWebSocket maintains a single live connection to the backend, validating every
// frame and reconnecting with exponential backoff.
export function useWebSocket(url: string = WS_URL): void {
  useEffect(() => {
    let socket: WebSocket | null = null
    let stopped = false
    let backoff = BACKOFF_MIN
    let timer: ReturnType<typeof setTimeout> | undefined
    const setConnection = useStreamStore.getState().setConnection

    const connect = () => {
      setConnection('connecting')
      socket = new WebSocket(url)
      socket.onopen = () => {
        backoff = BACKOFF_MIN
        setConnection('open')
      }
      socket.onmessage = (e) => dispatchFrame(e.data)
      socket.onerror = () => socket?.close()
      socket.onclose = () => {
        setConnection('closed')
        if (stopped) return
        timer = setTimeout(connect, backoff)
        backoff = Math.min(backoff * 2, BACKOFF_MAX)
      }
    }
    connect()

    return () => {
      stopped = true
      if (timer) clearTimeout(timer)
      socket?.close()
    }
  }, [url])
}
