// App is the application shell. It is built out across the topology, event-stream,
// RPKI sidebar, status-bar, and drill-down steps; for now it renders the observatory
// empty state so the design system can be verified.
export function App() {
  return (
    <main style={{ display: 'grid', placeItems: 'center', height: '100%' }} aria-label="BGPulse routing observatory">
      <div style={{ textAlign: 'center' }}>
        <h1 style={{ fontSize: 'var(--text-display)', fontWeight: 700, letterSpacing: '-0.02em' }}>
          BGPulse
        </h1>
        <p className="label" style={{ marginTop: 'var(--space-3)' }}>
          routing observatory · initializing
        </p>
      </div>
    </main>
  )
}
