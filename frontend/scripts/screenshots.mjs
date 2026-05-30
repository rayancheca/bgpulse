// Drives the running BGPulse app through its golden path and captures the README
// screenshots from real, live data. Requires the full stack running (the backend in
// demo mode serving the built frontend via -static-dir). Usage:
//   node scripts/screenshots.mjs   (BGP_URL defaults to http://localhost:8080)
import { chromium } from 'playwright'
import { mkdirSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const here = dirname(fileURLToPath(import.meta.url))
const OUT = join(here, '..', '..', 'docs', 'screenshots')
mkdirSync(OUT, { recursive: true })
const BASE = process.env.BGP_URL || 'http://localhost:8080'

const browser = await chromium.launch()
const page = await browser.newPage({ viewport: { width: 1600, height: 1000 }, deviceScaleFactor: 1.5 })

const shot = (name) => page.screenshot({ path: join(OUT, name) })
const stats = () => page.evaluate(async () => (await (await fetch('/api/stats')).json()).data)
async function until(pred, ms = 30000) {
  const start = Date.now()
  for (;;) {
    try {
      const s = await stats()
      if (s && pred(s)) return s
    } catch {
      // ignore transient fetch errors during startup
    }
    if (Date.now() - start > ms) throw new Error('timeout waiting for condition')
    await page.waitForTimeout(300)
  }
}
async function clip(selector, name) {
  const box = await page.evaluate((sel) => {
    const el = document.querySelector(sel)
    if (!el) return null
    const r = el.getBoundingClientRect()
    return { x: r.x, y: r.y, width: r.width, height: r.height }
  }, selector)
  if (box) await page.screenshot({ path: join(OUT, name), clip: box })
}

await page.goto(BASE, { waitUntil: 'domcontentloaded' })
await page.waitForSelector('.app-shell', { timeout: 8000 })
await shot('01-launch.png')

// Steady-state constellation: wait for the graph to populate and the force layout to settle.
await until((s) => s.nodeCount > 40)
await page.waitForTimeout(4000)
await shot('02-topology.png')

// A route leak in progress.
await until((s) => s.leaks > 0)
await page.waitForTimeout(500)
await shot('03-leak.png')

// A prefix hijack in progress.
await until((s) => s.hijacks > 0)
await page.waitForTimeout(300)
await shot('04-hijack.png')

// RPKI sidebar close-up.
await clip('.rpki-sidebar', '05-rpki-sidebar.png')

// Node hover tooltip.
const node = await page.evaluate(() => {
  const g = window.__bgpulse.topology.getState().graph
  let best = null
  for (const n of g.nodes.values()) {
    if (n.x != null && (!best || n.prefixCount > best.prefixCount)) best = n
  }
  return best ? { x: best.x, y: best.y } : null
})
if (node) {
  await page.mouse.move(node.x, node.y)
  await page.waitForTimeout(450)
  await shot('06-node-hover.png')
  await page.mouse.move(4, 4)
}

// Edge drill-down drawer — prefer an anomalous edge.
await page.evaluate(() => {
  const t = window.__bgpulse.topology.getState()
  let pick = null
  let anomaly = null
  for (const l of t.graph.links.values()) {
    if (!pick) pick = l.edgeKey
    if (l.status === 'hijack' || l.status === 'leak') anomaly = l.edgeKey
  }
  const key = anomaly || pick
  if (key) t.selectEdge(key)
})
await page.waitForTimeout(700)
await shot('07-drill-down.png')

// Event-stream rail close-up.
await page.evaluate(() => window.__bgpulse.topology.getState().selectEdge(null))
await page.waitForTimeout(300)
await clip('.event-rail', '08-event-rail.png')

await browser.close()
console.log('screenshots written to', OUT)
