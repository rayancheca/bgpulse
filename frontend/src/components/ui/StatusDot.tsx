interface StatusDotProps {
  color: string
  pulse?: boolean
  title?: string
}

// StatusDot is a small connection/status indicator dot.
export function StatusDot({ color, pulse = false, title }: StatusDotProps) {
  return (
    <span
      className={`status-dot${pulse ? ' status-dot--pulse' : ''}`}
      style={{ background: color, boxShadow: `0 0 8px ${color}` }}
      title={title}
      aria-hidden="true"
    />
  )
}
