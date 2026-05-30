import { useEffect, useRef, useState, type RefObject } from 'react'

export interface Size {
  width: number
  height: number
  dpr: number
}

// useResizeObserver tracks an element's content-box size and the device pixel ratio,
// for sizing a high-DPI canvas.
export function useResizeObserver<T extends HTMLElement>(): {
  ref: RefObject<T | null>
  size: Size
} {
  const ref = useRef<T>(null)
  const [size, setSize] = useState<Size>({ width: 0, height: 0, dpr: 1 })

  useEffect(() => {
    const el = ref.current
    if (!el) return
    const dpr = typeof window !== 'undefined' ? window.devicePixelRatio || 1 : 1
    const observer = new ResizeObserver((entries) => {
      const entry = entries[0]
      if (!entry) return
      const { width, height } = entry.contentRect
      setSize({ width, height, dpr })
    })
    observer.observe(el)
    return () => observer.disconnect()
  }, [])

  return { ref, size }
}
