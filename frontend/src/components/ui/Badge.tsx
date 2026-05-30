interface BadgeProps {
  label: string
  color?: string
  tone?: 'solid' | 'outline'
}

// Badge renders a small status tag (e.g. A/W kind, LEAK/HIJACK).
export function Badge({ label, color = 'var(--color-text-secondary)', tone = 'outline' }: BadgeProps) {
  const style =
    tone === 'solid'
      ? { background: color, color: 'var(--color-bg)', borderColor: color }
      : { color, borderColor: color }
  return (
    <span className="badge" style={style}>
      {label}
    </span>
  )
}
