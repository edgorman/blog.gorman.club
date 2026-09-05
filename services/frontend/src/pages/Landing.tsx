import { useEffect, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { FeedList } from '../components/FeedList'
import { useApp } from '../context/AppContext'
import { errorMessage, type Blog, type ListBlogsParams } from '../lib/api'

const FEED_SIZE = 10

/**
 * The active filters in the shape `listBlogs` takes, carrying only the ones actually set - so an
 * unfiltered feed asks for the feed rather than for one narrowed to nothing in particular.
 */
function filterParams(tag: string, query: string): ListBlogsParams {
  return { ...(tag ? { tag } : {}), ...(query ? { q: query } : {}) }
}

type State =
  | { phase: 'unconfigured' }
  | { phase: 'loading' }
  | { phase: 'error'; message: string }
  | { phase: 'ready'; posts: Blog[]; hasMore: boolean; loadingMore: boolean; loadMoreError?: string }

/**
 * The most recent posts the caller can read, newest first, one page at a time - the backend
 * paginates `GET /blogs` so this never fetches more than a screen's worth in one call.
 *
 * The two filters live in the query string (`?tag=`, `?q=`) rather than in component state, which
 * is what makes a filtered feed a place: it survives a reload, it can be linked to, and the tag
 * chips on every post can point at it. Both narrow the same feed and compose with each other, and
 * neither can widen it - a post the caller may not read is invisible to a search that names it
 * exactly.
 */
export function Landing() {
  const { api } = useApp()
  const [searchParams, setSearchParams] = useSearchParams()
  const tag = searchParams.get('tag') ?? ''
  const query = searchParams.get('q') ?? ''
  const filtered = tag !== '' || query !== ''

  const [state, setState] = useState<State>(api ? { phase: 'loading' } : { phase: 'unconfigured' })
  // What is in the box, which becomes `q` only once the reader submits it: typing must not refetch
  // the feed on every keystroke. It is put back in step during the render that notices the URL
  // changed beneath it - the Back button, or Clear - rather than from an effect, since an effect
  // would render the stale term once before correcting it.
  const [term, setTerm] = useState(query)
  const [lastQuery, setLastQuery] = useState(query)
  if (lastQuery !== query) {
    setLastQuery(query)
    setTerm(query)
  }

  useEffect(() => {
    if (!api) return
    setState({ phase: 'loading' })
    api
      .listBlogs({ limit: FEED_SIZE, ...filterParams(tag, query) })
      .then((page) => {
        setState({ phase: 'ready', posts: page.posts, hasMore: page.hasMore, loadingMore: false })
      })
      .catch((e: unknown) => {
        setState({ phase: 'error', message: errorMessage(e, 'Failed to load the feed') })
      })
  }, [api, tag, query])

  const loadMore = () => {
    if (!api || state.phase !== 'ready' || state.loadingMore) return
    const cursor = state.posts.at(-1)?.createdAt
    setState({ ...state, loadingMore: true, loadMoreError: undefined })

    api
      // The filters are carried into the next page as well: a cursor says where a feed was left
      // off, not what it was narrowed to.
      .listBlogs({ limit: FEED_SIZE, startAfter: cursor, ...filterParams(tag, query) })
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

  // Written as a whole new query string rather than as an edit to the old one, so that submitting
  // an empty box removes `q` instead of leaving it there empty - and so the tag beside it survives
  // either way.
  const setFilters = (next: { tag?: string; q?: string }) => {
    const params = new URLSearchParams()
    if (next.tag) params.set('tag', next.tag)
    if (next.q) params.set('q', next.q)
    setSearchParams(params)
  }

  return (
    <div className="page">
      <header className="page-header">
        <span className="page-kicker text-muted">Feed</span>
        <h1 className="title-feed">{tag ? `Posts tagged ${tag}` : 'Recent posts'}</h1>
      </header>

      <form
        className="feed-search"
        role="search"
        onSubmit={(e) => {
          e.preventDefault()
          setFilters({ tag, q: term.trim() })
        }}
      >
        <input
          className="input"
          type="search"
          aria-label="Search posts"
          placeholder="Search posts"
          value={term}
          onChange={(e) => setTerm(e.target.value)}
        />
        <button type="submit" className="btn btn-secondary">
          Search
        </button>
        {filtered && (
          <button type="button" className="btn btn-ghost" onClick={() => setFilters({})}>
            Clear
          </button>
        )}
      </form>

      {state.phase === 'unconfigured' && (
        <p className="text-muted">No backend deployed yet - VITE_BACKEND_URL is unset.</p>
      )}
      {state.phase === 'loading' && <p className="text-muted">Loading…</p>}
      {state.phase === 'error' && <p role="alert">{state.message}</p>}
      {state.phase === 'ready' && state.posts.length === 0 && (
        <p className="text-muted">
          {filtered ? 'No posts match that.' : 'No posts yet. Be the first to write something.'}
        </p>
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
