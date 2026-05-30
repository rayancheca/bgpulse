import { useStreamStore } from '../hooks/useStreamStore'
import { EdgeDrawer } from '../components/drill-down/EdgeDrawer'
import { EventRail } from '../components/event-stream/EventRail'
import { RpkiSidebar } from '../components/rpki-sidebar/RpkiSidebar'
import { StatusBar } from '../components/status-bar/StatusBar'
import { TopologyCanvas } from '../components/topology/TopologyCanvas'
import './app-shell.css'

// AppShell is the HUD layout: the topology fills the canvas edge-to-edge and every
// other surface floats over it.
export function AppShell() {
  const connection = useStreamStore((s) => s.connection)
  const hasData = useStreamStore((s) => s.stats !== null || s.events.length > 0)

  return (
    <div className="app-shell">
      <TopologyCanvas />
      <div className="app-shell__topbar">
        <StatusBar />
      </div>
      <EventRail />
      <RpkiSidebar />
      <EdgeDrawer />
      {!hasData ? (
        <div className="app-shell__empty">
          <div className="app-shell__empty-title">BGPulse</div>
          <div className="label">
            {connection === 'open'
              ? 'awaiting the BGP stream…'
              : connection === 'connecting'
                ? 'connecting to the routing observatory…'
                : 'reconnecting…'}
          </div>
        </div>
      ) : null}
    </div>
  )
}
