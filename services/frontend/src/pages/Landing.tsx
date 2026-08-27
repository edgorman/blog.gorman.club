import { useEffect, useState } from 'react'
import { FeedList } from '../components/FeedList'
import { useApp } from '../context/AppContext'
import { errorMessage, type Blog } from '../lib/api'

const FEED_SIZE = 10

type State =
  | { phase: 'unconfigured' }
  | { phase: 'loading' }
  | { phase: 'error'; message: string }
  | { phase: 'ready'; posts: Blog[] }

/** The most recent public posts across every author, newest first. */
export function Landing() {
  const { api } = useApp()
  const [state, setState] = useState<State>(api ? { phase: 'loading' } : { phase: 'unconfigured' })

  useEffect(() => {
    if (!api) return
    setState({ phase: 'loading' })
    api
      .listBlogs()
      .then((posts) => {
        const recent = [...posts]
          .sort((a, b) => b.createdAt.localeCompare(a.createdAt))
          .slice(0, FEED_SIZE)
        setState({ phase: 'ready', posts: recent })
      })
      .catch((e: unknown) => {
        setState({ phase: 'error', message: errorMessage(e, 'Failed to load the feed') })
      })
  }, [api])

  return (
    <div className="page">
      <header className="page-header">
        <span className="page-kicker text-muted">Feed</span>
        <h1 className="title-feed">Recent posts</h1>
      </header>

      {state.phase === 'unconfigured' && (
        <p className="text-muted">No backend deployed yet - VITE_BACKEND_URL is unset.</p>
      )}
      {state.phase === 'loading' && <p className="text-muted">Loading…</p>}
      {state.phase === 'error' && <p role="alert">{state.message}</p>}
      {state.phase === 'ready' && state.posts.length === 0 && (
        <p className="text-muted">No posts yet. Be the first to write something.</p>
      )}
      {state.phase === 'ready' && state.posts.length > 0 && <FeedList posts={state.posts} />}
    </div>
  )
}
