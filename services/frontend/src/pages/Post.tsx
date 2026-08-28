import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { useApp } from '../context/AppContext'
import { ApiError, postPath, type Blog } from '../lib/api'
import { formatDate } from '../lib/format'
import { renderMarkdown } from '../lib/markdown'

type State =
  | { phase: 'unconfigured' }
  | { phase: 'loading' }
  | { phase: 'not-found' }
  | { phase: 'forbidden' }
  | { phase: 'error'; message: string }
  | { phase: 'ready'; post: Blog }

export function Post() {
  // Both halves address the post: a slug belongs to one author, so neither identifies it alone.
  const { username, slug } = useParams<{ username: string; slug: string }>()
  const { api, user } = useApp()
  const [state, setState] = useState<State>(api ? { phase: 'loading' } : { phase: 'unconfigured' })

  useEffect(() => {
    if (!api || !username || !slug) return
    setState({ phase: 'loading' })
    api
      .getBlog(username, slug)
      .then((post) => setState({ phase: 'ready', post }))
      .catch((e: unknown) => {
        if (e instanceof ApiError && e.status === 404) return setState({ phase: 'not-found' })
        if (e instanceof ApiError && e.status === 403) return setState({ phase: 'forbidden' })
        setState({ phase: 'error', message: e instanceof Error ? e.message : 'Failed to load post' })
      })
  }, [api, username, slug])

  useEffect(() => {
    if (state.phase !== 'ready') return
    const hash = window.location.hash.slice(1)
    if (!hash) return
    const target = decodeURIComponent(hash)
    const timeoutId = window.setTimeout(() => {
      // Some markdown sources target legacy `<a name="...">` anchors rather than an element id.
      const el = document.getElementById(target) ?? document.getElementsByName(target)[0]
      el?.scrollIntoView()
    }, 0)
    return () => window.clearTimeout(timeoutId)
  }, [state.phase])

  if (state.phase === 'unconfigured') {
    return (
      <div className="page">
        <p className="text-muted center-note">No backend deployed yet - VITE_BACKEND_URL is unset.</p>
      </div>
    )
  }
  if (state.phase === 'loading') {
    return (
      <div className="page">
        <p className="text-muted center-note">Loading…</p>
      </div>
    )
  }
  if (state.phase === 'not-found') {
    return (
      <div className="page">
        <p className="center-note">Post not found.</p>
        <Link to="/">← Back to feed</Link>
      </div>
    )
  }
  if (state.phase === 'forbidden') {
    return (
      <div className="page">
        <p className="center-note">This post is private.</p>
        <Link to="/">← Back to feed</Link>
      </div>
    )
  }
  if (state.phase === 'error') {
    return (
      <div className="page">
        <p role="alert" className="center-note">
          {state.message}
        </p>
      </div>
    )
  }

  const { post } = state
  // A post whose owner never set up a profile has no username, and so no profile page to link to:
  // without one there is no address for it.
  const authorName = post.authorUsername || 'an unnamed author'
  const authorHref = post.authorUsername ? `/profile/${post.authorUsername}` : null
  // The post was reached through its own address, so it has one; the guard is for the type, not
  // for a case this page can actually be in.
  const href = postPath(post)

  return (
    <div className="page">
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 'var(--space-3)' }}>
        {authorHref ? (
          <Link to={authorHref} className="text-muted back-link">
            ← Back to {authorName}&apos;s posts
          </Link>
        ) : (
          <Link to="/" className="text-muted back-link">
            ← Back to feed
          </Link>
        )}
        {user?.id === post.ownerId && href && (
          <Link to={`${href}/edit`} className="btn btn-secondary">
            Edit
          </Link>
        )}
      </div>
      <header style={{ paddingBottom: 'var(--space-4)' }}>
        <div className="post-meta">
          <span className="text-muted feed-row-date">{formatDate(post.createdAt)}</span>
          {post.visibility === 'private' && <span className="tag tag-outline">private</span>}
        </div>
        <h1 className="title-post">{post.title || '(untitled)'}</h1>
        {authorHref ? (
          <Link to={authorHref} className="text-muted post-author">
            by {authorName}
          </Link>
        ) : (
          <span className="text-muted post-author">by {authorName}</span>
        )}
      </header>
      <hr className="hr" />
      <div className="post-body" dangerouslySetInnerHTML={{ __html: renderMarkdown(post.content) }} />
    </div>
  )
}
