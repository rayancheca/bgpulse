import { create } from 'zustand'

import { MAX_EVENTS } from '../lib/constants'
import type { ClassifiedEvent, Stats } from '../lib/types'

export type Connection = 'connecting' | 'open' | 'closed'

interface StreamState {
  events: ClassifiedEvent[] // newest first, bounded to MAX_EVENTS
  stats: Stats | null
  connection: Connection
  pushEvent: (ev: ClassifiedEvent) => void
  setEvents: (events: ClassifiedEvent[]) => void
  setStats: (stats: Stats) => void
  setConnection: (c: Connection) => void
}

// useStreamStore holds the bounded recent-event ring, the latest stats, and the
// connection status. The event rail, status bar, and RPKI sidebar subscribe to it.
export const useStreamStore = create<StreamState>()((set) => ({
  events: [],
  stats: null,
  connection: 'connecting',
  pushEvent: (ev) =>
    set((s) => {
      const events = [ev, ...s.events]
      if (events.length > MAX_EVENTS) events.length = MAX_EVENTS
      return { events }
    }),
  setEvents: (events) => set({ events: events.slice(0, MAX_EVENTS) }),
  setStats: (stats) => set({ stats }),
  setConnection: (connection) => set({ connection }),
}))
