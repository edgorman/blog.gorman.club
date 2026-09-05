import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { Comments } from '../components/Comments'
import { ReactionBar } from '../components/ReactionBar'
import { TagList } from '../components/TagList'
import { useApp } from '../context/AppContext'
import { useReactions } from '../hooks/useReactions'
import { ApiError, postPath, userPath, type Blog } from '../lib/api'
import { formatDate } from '../lib/format'
import { renderMarkdown } from '../lib/markdown'

type State =
  | { phase: 'unconfigured' }
  | { phase: 'loading' }
  // The backend answers a private post the caller cannot read the same way it answers a missing
  // one, so there is no separate forbidden state here to render.
  | { phase: 'not-found' }
  | { phase: 'error'; message: string }
  | { phase: 'ready'; post: Blog }

export function Post() {
  // The slug addresses the post on its own: slugs are unique across every author.
  const { slug } = useParams<{ slug: string }>()
  const { api, user } = useApp()
  const [state, setState] = useState<State>(api ? { phase: 'loading' } : { phase: 'unconfigured' })
  // Loaded for the whole page at once - the post's reactions and every comment's come back
  // together - so this lives here rather than inside the two components that draw them.
  const reactions = useReactions(slug ?? '')

  useEffect(() => {
    if (!api || !slug) return
    setState({ phase: 'loading' })
    api
      .getBlog(slug)
      .then((post) => setState({ phase: 'ready', post }))
      .catch((e: unknown) => {
        if (e instanceof ApiError && e.status === 404) return setState({ phase: 'not-found' })
        setState({ phase: 'error', message: e instanceof Error ? e.message : 'Failed to load post' })
      })
  }, [api, slug])

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
  const authorHref = userPath(post.authorUsername)
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
        {user?.id === post.ownerId && (
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
        {/* Followed rather than only read, unlike the feed's: this header is not itself a link,
            and a tag here is the most natural way into the rest of what an author wrote on it. */}
        <TagList tags={post.tags} linked />
      </header>
      <hr className="hr" />
      <div className="post-body" dangerouslySetInnerHTML={{ __html: renderMarkdown(post.content) }} />
      <ReactionBar
        counts={reactions.countsFor()}
        onToggle={(emoji) => reactions.toggle(emoji)}
        canReact={!!user}
        label="post"
      />
      {reactions.error && (
        <p role="alert" className="reactions-error">
          {reactions.error}
        </p>
      )}
      {/* The thread is as visible as the post: this only renders for a post the caller could read
          in the first place, and the backend applies the same rule to the comments themselves. */}
      <Comments slug={post.slug} ownerId={post.ownerId} reactions={reactions} />
    </div>
  )
}
