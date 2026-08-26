import { useEffect, useState } from 'react'
import { errorMessage, type Api, type Blog } from '../lib/api'

interface Props {
  api: Api
  uid: string
}

type Draft = Pick<Blog, 'title' | 'content' | 'visibility'>

const EMPTY: Draft = { title: '', content: '', visibility: 'public' }

/** Exercises the full /blogs surface: list, create, update, delete. */
export function Blogs({ api, uid }: Props) {
  const [blogs, setBlogs] = useState<Blog[]>([])
  const [draft, setDraft] = useState<Draft>(EMPTY)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [status, setStatus] = useState('Loading…')

  const reload = () => {
    api
      .listBlogs()
      .then((list) => {
        setBlogs(list)
        setStatus(`${list.length} readable blog${list.length === 1 ? '' : 's'}`)
      })
      .catch((e: unknown) => setStatus(errorMessage(e, 'Failed to list blogs')))
  }

  useEffect(reload, [api])

  const submit = () => {
    setStatus('Saving…')
    const request = editingId ? api.updateBlog(editingId, draft) : api.createBlog(draft)

    request
      .then(() => {
        setDraft(EMPTY)
        setEditingId(null)
        reload()
      })
      .catch((e: unknown) => setStatus(errorMessage(e, 'Save failed')))
  }

  const remove = (id: string) => {
    setStatus('Deleting…')
    api
      .deleteBlog(id)
      .then(reload)
      .catch((e: unknown) => setStatus(errorMessage(e, 'Delete failed')))
  }

  const edit = (blog: Blog) => {
    setEditingId(blog.id)
    setDraft({ title: blog.title, content: blog.content, visibility: blog.visibility })
  }

  return (
    <section className="panel">
      <h2>Blogs</h2>
      <p className="status">{status}</p>

      <label>
        Title
        <input value={draft.title} onChange={(e) => setDraft({ ...draft, title: e.target.value })} />
      </label>
      <label>
        Content
        <textarea
          rows={3}
          value={draft.content}
          onChange={(e) => setDraft({ ...draft, content: e.target.value })}
        />
      </label>
      <label>
        Visibility
        <select
          value={draft.visibility}
          onChange={(e) => setDraft({ ...draft, visibility: e.target.value as Draft['visibility'] })}
        >
          <option value="public">public</option>
          <option value="private">private</option>
        </select>
      </label>

      <div className="actions">
        <button onClick={submit}>{editingId ? 'Update' : 'Create'}</button>
        {editingId && (
          <button
            onClick={() => {
              setEditingId(null)
              setDraft(EMPTY)
            }}
          >
            Cancel
          </button>
        )}
        <button onClick={reload}>Refresh</button>
      </div>

      <ul className="blog-list">
        {blogs.map((blog) => {
          const owned = blog.ownerId === uid
          return (
            <li key={blog.id}>
              <div>
                <strong>{blog.title || '(untitled)'}</strong>{' '}
                <span className="tag">{blog.visibility}</span>
                {!owned && <span className="tag">someone else's</span>}
                <p>{blog.content}</p>
              </div>
              {/* Only the owner may write, so the API would 403 these for anyone else. */}
              {owned && (
                <div className="actions">
                  <button onClick={() => edit(blog)}>Edit</button>
                  <button onClick={() => remove(blog.id)}>Delete</button>
                </div>
              )}
            </li>
          )
        })}
      </ul>
    </section>
  )
}
