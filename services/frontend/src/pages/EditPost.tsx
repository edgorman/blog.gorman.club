import { useEffect, useState } from 'react'
import { Link, Navigate, useParams } from 'react-router-dom'
import { AssistantPanel } from '../components/AssistantPanel'
import { useApp } from '../context/AppContext'
import { ApiError, errorMessage, postPath, type Blog } from '../lib/api'
import { renderMarkdown } from '../lib/markdown'

type State =
  | { phase: 'unconfigured' }
  | { phase: 'loading' }
  // The backend answers a private post the caller cannot read the same way it answers a missing
  // one, so there is no separate forbidden state here to render.
  | { phase: 'not-found' }
  | { phase: 'error'; message: string }
  | { phase: 'ready'; post: Blog }

type Mode = 'write' | 'preview'
type Visibility = Blog['visibility']

export function EditPost() {
  // The slug addresses the post on its own: slugs are unique across every author.
  const { slug } = useParams<{ slug: string }>()
  const { api, user, profile } = useApp()
  const [state, setState] = useState<State>(api ? { phase: 'loading' } : { phase: 'unconfigured' })
  const [title, setTitle] = useState('')
  const [markdown, setMarkdown] = useState('')
  const [visibility, setVisibility] = useState<Visibility>('public')
  const [mode, setMode] = useState<Mode>('write')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [saved, setSaved] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [deleted, setDeleted] = useState(false)

  useEffect(() => {
    if (!api || !slug) return
    setState({ phase: 'loading' })
    api
      .getBlog(slug)
      .then((post) => {
        setState({ phase: 'ready', post })
        setTitle(post.title)
        setMarkdown(post.content)
        setVisibility(post.visibility)
      })
      .catch((e: unknown) => {
        if (e instanceof ApiError && e.status === 404) return setState({ phase: 'not-found' })
        setState({ phase: 'error', message: e instanceof Error ? e.message : 'Failed to load post' })
      })
  }, [api, slug])

  const save = () => {
    if (!api || !slug || state.phase !== 'ready') return
    setSaving(true)
    setError(null)
    api
      // updateBlog is a full replace, so the private-post whitelist has to be carried through even
      // though this editor doesn't expose it for editing.
      // The slug is not editable, so a retitle saves against the address the post already has.
      .updateBlog(slug, {
        title,
        content: markdown,
        visibility,
        allowedUserIds: state.post.allowedUserIds,
      })
      .then(() => setSaved(true))
      .catch((e: unknown) => setError(errorMessage(e, 'Failed to save')))
      .finally(() => setSaving(false))
  }

  // The backend soft-deletes a post rather than erasing it, but nothing here lets the owner get it
  // back, so the prompt still has to warn them as if it were permanent.
  const remove = () => {
    if (!api || !slug) return
    if (!window.confirm('Delete this post? This cannot be undone.')) return
    setDeleting(true)
    setError(null)
    api
      .deleteBlog(slug)
      .then(() => setDeleted(true))
      .catch((e: unknown) => setError(errorMessage(e, 'Failed to delete')))
      .finally(() => setDeleting(false))
  }

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

  // A public post owned by someone else is fetched fine by getBlog, so ownership needs its own check.
  if (post.ownerId !== user?.id) {
    return (
      <div className="page">
        <p className="center-note">You don't have permission to edit this post.</p>
        <Link to={postPath(post)}>← Back to post</Link>
      </div>
    )
  }

  if (saved) return <Navigate to={postPath(post)} replace />
  if (deleted) return <Navigate to="/" replace />

  return (
    <div className="page">
      <header className="page-header">
        <span className="page-kicker text-muted">Edit post</span>
        <h1 className="title-editor">Edit your post</h1>
      </header>

      <div className="field">
        <label htmlFor="gc-title">Title</label>
        <input
          id="gc-title"
          className="input"
          placeholder="Untitled post"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
        />
      </div>

      <div className="seg" role="radiogroup" aria-label="Visibility" style={{ marginBottom: 'var(--space-3)' }}>
        <label className="seg-opt">
          <input
            type="radio"
            name="visibility"
            checked={visibility === 'public'}
            onChange={() => setVisibility('public')}
          />
          Public
        </label>
        <label className="seg-opt">
          <input
            type="radio"
            name="visibility"
            checked={visibility === 'private'}
            onChange={() => setVisibility('private')}
          />
          Private
        </label>
      </div>

      <div className="seg" role="radiogroup" aria-label="Editor mode" style={{ marginBottom: 'var(--space-3)' }}>
        <label className="seg-opt">
          <input type="radio" name="mode" checked={mode === 'write'} onChange={() => setMode('write')} />
          Write
        </label>
        <label className="seg-opt">
          <input type="radio" name="mode" checked={mode === 'preview'} onChange={() => setMode('preview')} />
          Preview
        </label>
      </div>

      <div className={profile?.assistantEnabled ? 'editor-with-assistant' : undefined}>
        {mode === 'write' ? (
          <div className="gc-pane">
            <textarea
              className="gc-editor"
              placeholder="Write in markdown..."
              value={markdown}
              onChange={(e) => setMarkdown(e.target.value)}
            />
          </div>
        ) : (
          <div
            className="gc-pane post-body"
            style={{ minHeight: 360, padding: 'var(--space-3) 0' }}
            dangerouslySetInnerHTML={{ __html: renderMarkdown(markdown) }}
          />
        )}

        {/* The assistant edits the post server-side, so what it leaves behind replaces what is in
            these fields - otherwise the next save would write the pre-assistant text back over it.
            The panel is only rendered for an account the backend would actually let use it; the
            routes enforce that either way. */}
        {profile?.assistantEnabled && (
          <AssistantPanel
            slug={post.slug}
            title={title}
            content={markdown}
            onEdited={(draft) => {
              setTitle(draft.title)
              setMarkdown(draft.content)
            }}
          />
        )}
      </div>

      {error && (
        <p role="alert" style={{ marginTop: 'var(--space-3)' }}>
          {error}
        </p>
      )}

      <div style={{ display: 'flex', gap: 'var(--space-3)', marginTop: 'var(--space-4)' }}>
        <button type="button" className="btn btn-primary" onClick={save} disabled={saving}>
          {saving ? 'Saving…' : 'Save'}
        </button>
        <Link to={postPath(post)} className="btn btn-secondary">
          Cancel
        </Link>
        <button type="button" className="btn btn-danger" onClick={remove} disabled={deleting}>
          {deleting ? 'Deleting…' : 'Delete'}
        </button>
      </div>
    </div>
  )
}
