import { useState } from 'react'
import type { BlogInput } from '../lib/blogsApi'
import type { Blog } from '../lib/blogs'

interface BlogFormProps {
  initial?: Blog
  onSubmit: (input: BlogInput) => Promise<void>
  onCancel?: () => void
}

function parseAllowedUserIds(raw: string): string[] | undefined {
  const ids = raw
    .split(',')
    .map((id) => id.trim())
    .filter(Boolean)
  return ids.length > 0 ? ids : undefined
}

export function BlogForm({ initial, onSubmit, onCancel }: BlogFormProps) {
  const [title, setTitle] = useState(initial?.title ?? '')
  const [content, setContent] = useState(initial?.content ?? '')
  const [visibility, setVisibility] = useState<Blog['visibility']>(
    initial?.visibility ?? 'public',
  )
  const [allowedUserIds, setAllowedUserIds] = useState((initial?.allowedUserIds ?? []).join(', '))
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault()
    setSubmitting(true)
    setError(null)

    try {
      await onSubmit({
        title,
        content,
        visibility,
        allowedUserIds: visibility === 'private' ? parseAllowedUserIds(allowedUserIds) : undefined,
      })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form onSubmit={handleSubmit}>
      <label>
        Title
        <input value={title} onChange={(e) => setTitle(e.target.value)} required />
      </label>

      <label>
        Content
        <textarea value={content} onChange={(e) => setContent(e.target.value)} required />
      </label>

      <label>
        Visibility
        <select
          value={visibility}
          onChange={(e) => setVisibility(e.target.value as Blog['visibility'])}
        >
          <option value="public">Public</option>
          <option value="private">Private</option>
        </select>
      </label>

      {visibility === 'private' && (
        <label>
          Allowed user IDs (comma-separated)
          <input value={allowedUserIds} onChange={(e) => setAllowedUserIds(e.target.value)} />
        </label>
      )}

      {error && <p role="alert">{error}</p>}

      <button type="submit" disabled={submitting}>
        {initial ? 'Save' : 'Create'}
      </button>
      {onCancel && (
        <button type="button" onClick={onCancel} disabled={submitting}>
          Cancel
        </button>
      )}
    </form>
  )
}
