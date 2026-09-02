import { useEffect, useState } from 'react'
import { FeedList } from '../components/FeedList'
import { useApp } from '../context/AppContext'
import { errorMessage, type Blog } from '../lib/api'

const FEED_SIZE = 10

type State =
  | { phase: 'unconfigured' }
  | { phase: 'loading' }
  | { phase: 'error'; message: string }
  | { phase: 'ready'; posts: Blog[]; hasMore: boolean; loadingMore: boolean; loadMoreError?: string }

/**
 * The most recent public posts across every author, newest first, one page at a time - the
 * backend paginates `GET /blogs` so this never fetches more than a screen's worth in one call.
 */
export function Landing() {
  const { api } = useApp()
  const [state, setState] = useState<State>(api ? { phase: 'loading' } : { phase: 'unconfigured' })

  useEffect(() => {
    if (!api) return
    setState({ phase: 'loading' })
    api
      .listBlogs({ limit: FEED_SIZE })
      .then((page) => {
        setState({ phase: 'ready', posts: page.posts, hasMore: page.hasMore, loadingMore: false })
      })
      .catch((e: unknown) => {
        setState({ phase: 'error', message: errorMessage(e, 'Failed to load the feed') })
      })
  }, [api])

  const loadMore = () => {
    if (!api || state.phase !== 'ready' || state.loadingMore) return
    const cursor = state.posts.at(-1)?.createdAt
    setState({ ...state, loadingMore: true, loadMoreError: undefined })

    api
      .listBlogs({ limit: FEED_SIZE, startAfter: cursor })
      .then((page) => {
        setState((prev) =>
          prev.phase === 'ready'
            ? { phase: 'ready', posts: [...prev.posts, ...page.posts], hasMore: page.hasMore, loadingMore: false }
            : prev,
        )
      })
      .catch((e: unknown) => {
        setState((prev) =>
          prev.phase === 'ready'
            ? { ...prev, loadingMore: false, loadMoreError: errorMessage(e, 'Failed to load more posts') }
            : prev,
        )
      })
  }

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
      {state.phase === 'ready' && state.posts.length > 0 && (
        <>
          <FeedList posts={state.posts} />
          {(state.hasMore || state.loadMoreError) && (
            <div className="feed-load-more">
              {state.loadMoreError && <p role="alert">{state.loadMoreError}</p>}
              {state.hasMore && (
                <button type="button" className="btn btn-ghost" onClick={loadMore} disabled={state.loadingMore}>
                  {state.loadingMore ? 'Loading…' : 'Load more'}
                </button>
              )}
            </div>
          )}
        </>
      )}
    </div>
  )
}
