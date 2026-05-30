import type { ReactNode } from 'react'
import './ui.css'

interface PanelProps {
  className?: string
  children: ReactNode
  'aria-label'?: string
}

// Panel is the translucent, blurred floating-overlay primitive used for every chrome
// surface docked over the topology canvas.
export function Panel({ className = '', children, ...rest }: PanelProps) {
  return (
    <div className={`panel ${className}`} {...rest}>
      {children}
    </div>
  )
}
