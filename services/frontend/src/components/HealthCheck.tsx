import { useEffect, useState } from 'react'
import { fetchHealth, type HealthStatus } from '../lib/health'

const BACKEND_URL = import.meta.env.VITE_BACKEND_URL

type State =
  | { phase: 'unconfigured' }
  | { phase: 'loading' }
  | { phase: 'success'; data: HealthStatus }
  | { phase: 'error'; message: string }

export function HealthCheck() {
  const [state, setState] = useState<State>(
    BACKEND_URL ? { phase: 'loading' } : { phase: 'unconfigured' },
  )

  useEffect(() => {
    if (!BACKEND_URL) return

    const controller = new AbortController()

    fetchHealth(BACKEND_URL, controller.signal)
      .then((data) => setState({ phase: 'success', data }))
      .catch((error: unknown) => {
        if (controller.signal.aborted) return
        const message = error instanceof Error ? error.message : 'Unknown error'
        setState({ phase: 'error', message })
      })

    return () => controller.abort()
  }, [])

  return (
    <div className="health-check" data-phase={state.phase}>
      <h2>Backend status</h2>

      {state.phase === 'unconfigured' && (
        <p>
          No backend deployed yet. Once <code>VITE_BACKEND_URL</code> is set, this card will
          call the backend's <code>/debug</code> endpoint on page load.
        </p>
      )}

      {state.phase === 'loading' && <p>Checking backend health…</p>}

      {state.phase === 'error' && (
        <p role="alert">Backend health check failed: {state.message}</p>
      )}

      {state.phase === 'success' && (
        <dl>
          <dt>Status</dt>
          <dd>{state.data.status}</dd>
          <dt>Environment</dt>
          <dd>{state.data.environment}</dd>
          <dt>Commit</dt>
          <dd>{state.data.commit}</dd>
          <dt>Timestamp</dt>
          <dd>{state.data.timestamp}</dd>
        </dl>
      )}
    </div>
  )
}
