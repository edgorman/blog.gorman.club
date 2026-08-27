import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { useApp } from '../context/AppContext'
import { ApiError, type Blog } from '../lib/api'
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
  const { id } = useParams<{ id: string }>()
  const { api, resolveAuthorName } = useApp()
  const [state, setState] = useState<State>(api ? { phase: 'loading' } : { phase: 'unconfigured' })
  const [authorName, setAuthorName] = useState<string | null>(null)

  useEffect(() => {
    if (!api || !id) return
    setState({ phase: 'loading' })
    api
      .getBlog(id)
      .then((post) => setState({ phase: 'ready', post }))
      .catch((e: unknown) => {
        if (e instanceof ApiError && e.status === 404) return setState({ phase: 'not-found' })
        if (e instanceof ApiError && e.status === 403) return setState({ phase: 'forbidden' })
        setState({ phase: 'error', message: e instanceof Error ? e.message : 'Failed to load post' })
      })
  }, [api, id])

  const ownerId = state.phase === 'ready' ? state.post.ownerId : null

  useEffect(() => {
    if (!ownerId) return
    let cancelled = false
    resolveAuthorName(ownerId).then((name) => {
      if (!cancelled) setAuthorName(name)
    })
    return () => {
      cancelled = true
    }
  }, [ownerId, resolveAuthorName])

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
  const authorHref = `/profile/${post.ownerId}`

  return (
    <div className="page">
      <Link to={authorHref} className="text-muted back-link">
        ← Back to {authorName ?? 'author'}
      </Link>
      <header style={{ paddingBottom: 'var(--space-4)' }}>
        <div className="post-meta">
          <span className="text-muted feed-row-date">{formatDate(post.createdAt)}</span>
          {post.visibility === 'private' && <span className="tag tag-outline">private</span>}
        </div>
        <h1 className="title-post">{post.title || '(untitled)'}</h1>
        <Link to={authorHref} className="text-muted post-author">
          by {authorName ?? '…'}
        </Link>
      </header>
      <hr className="hr" />
      <div className="post-body" dangerouslySetInnerHTML={{ __html: renderMarkdown(post.content) }} />
    </div>
  )
}
