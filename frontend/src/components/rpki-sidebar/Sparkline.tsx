interface SparklineProps {
  data: number[]
  color: string
  width?: number
  height?: number
}

// Sparkline draws a tiny throughput area chart for an origin AS.
export function Sparkline({ data, color, width = 120, height = 22 }: SparklineProps) {
  if (data.length === 0) {
    return <svg className="sparkline" width={width} height={height} aria-hidden="true" />
  }
  const max = Math.max(1, ...data)
  const step = data.length > 1 ? width / (data.length - 1) : width
  const line = data
    .map((v, i) => {
      const x = i * step
      const y = height - (v / max) * (height - 2) - 1
      return `${i === 0 ? 'M' : 'L'}${x.toFixed(1)},${y.toFixed(1)}`
    })
    .join(' ')
  const area = `${line} L${width.toFixed(1)},${height} L0,${height} Z`
  return (
    <svg className="sparkline" width={width} height={height} preserveAspectRatio="none" aria-hidden="true">
      <path d={area} fill={color} opacity={0.14} />
      <path d={line} fill="none" stroke={color} strokeWidth={1.5} strokeLinejoin="round" />
    </svg>
  )
}
