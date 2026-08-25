import { useEffect, useRef } from 'react'

interface Props {
  ready: boolean
  onRender: (element: HTMLElement) => void
}

/** Host element for the button Google Identity Services renders into. */
export function GoogleSignInButton({ ready, onRender }: Props) {
  const containerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (ready && containerRef.current) {
      onRender(containerRef.current)
    }
  }, [ready, onRender])

  return <div ref={containerRef} />
}
