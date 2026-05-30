import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'

import '@fontsource/space-grotesk/400.css'
import '@fontsource/space-grotesk/500.css'
import '@fontsource/space-grotesk/700.css'
import '@fontsource/ibm-plex-mono/400.css'
import '@fontsource/ibm-plex-mono/500.css'

import './styles/tokens.css'
import './styles/typography.css'
import './styles/global.css'

import { App } from './app/App'

const root = document.getElementById('root')
if (!root) {
  throw new Error('root element not found')
}
createRoot(root).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
