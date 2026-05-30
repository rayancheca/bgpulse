import { AppShell } from './AppShell'
import { Providers } from './providers'

// App connects the live stream and renders the routing-observatory shell.
export function App() {
  return (
    <Providers>
      <AppShell />
    </Providers>
  )
}
