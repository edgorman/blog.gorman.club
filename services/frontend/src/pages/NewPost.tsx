import { useState } from 'react'
import { Link } from 'react-router-dom'
import { GoogleSignInButton } from '../components/GoogleSignInButton'
import { useApp } from '../context/AppContext'
import { errorMessage, type Blog } from '../lib/api'
import { renderMarkdown } from '../lib/markdown'

const STARTER_MD = `# Your title here

Start writing in **markdown**. A blank line makes a new paragraph.

- lists work
- so do \`inline code\` and > quotes

Switch to Preview any time to see the rendered post.`

type Mode = 'write' | 'preview'

export function NewPost() {
  const { api, user, authError, authReady, renderSignInButton } = useApp()
  const [title, setTitle] = useState('')
  const [markdown, setMarkdown] = useState(STARTER_MD)
  const [mode, setMode] = useState<Mode>('write')
  const [publishing, setPublishing] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [published, setPublished] = useState<Blog | null>(null)

  const publish = () => {
    if (!api) return
    setPublishing(true)
    setError(null)
    api
      .createBlog({ title, content: markdown, visibility: 'public' })
      .then(setPublished)
      .catch((e: unknown) => setError(errorMessage(e, 'Failed to publish')))
      .finally(() => setPublishing(false))
  }

  return (
    <div className="page">
      <header className="page-header">
        <span className="page-kicker text-muted">New post</span>
        <h1 className="title-editor">Write something</h1>
      </header>

      {!api && <p className="text-muted">No backend deployed yet - VITE_BACKEND_URL is unset.</p>}

      {api && !user && (
        <div className="center-note">
          <p className="text-muted">Sign in to publish a post.</p>
          {authError ? <p role="alert">{authError}</p> : <GoogleSignInButton ready={authReady} onRender={renderSignInButton} />}
        </div>
      )}

      {api && user && published && (
        <div className="center-note">
          <p style={{ fontSize: 18, margin: '0 0 var(--space-3)' }}>Published.</p>
          <p className="text-muted" style={{ margin: '0 0 var(--space-4)' }}>
            "{published.title || 'Untitled post'}" is live.
          </p>
          <div style={{ display: 'flex', gap: 'var(--space-3)', flexWrap: 'wrap' }}>
            <Link to={`/post/${published.id}`} className="btn btn-primary">
              View post
            </Link>
            <Link to="/" className="btn btn-secondary">
              Back to feed
            </Link>
          </div>
        </div>
      )}

      {api && user && !published && (
        <>
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

          {error && (
            <p role="alert" style={{ marginTop: 'var(--space-3)' }}>
              {error}
            </p>
          )}

          <div style={{ display: 'flex', gap: 'var(--space-3)', marginTop: 'var(--space-4)' }}>
            <button type="button" className="btn btn-primary" onClick={publish} disabled={publishing}>
              {publishing ? 'Publishing…' : 'Publish'}
            </button>
          </div>
        </>
      )}
    </div>
  )
}
