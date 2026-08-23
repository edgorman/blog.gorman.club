import { useCallback, useEffect, useState } from 'react'
import { authReady } from '../lib/firebase'
import { fetchVisibleBlogs, type Blog } from '../lib/blogs'
import { createBlog, deleteBlog, updateBlog, type BlogInput } from '../lib/blogsApi'
import { BlogForm } from './BlogForm'

export function Blogs() {
  const [uid, setUid] = useState<string | null>(null)
  const [blogs, setBlogs] = useState<Blog[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [editing, setEditing] = useState<string | null>(null)
  const [creating, setCreating] = useState(false)

  const reload = useCallback(async (signedInAs: string | null) => {
    try {
      setBlogs(await fetchVisibleBlogs(signedInAs))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error')
    }
  }, [])

  useEffect(() => {
    authReady
      .then(async (signedInAs) => {
        setUid(signedInAs)
        await reload(signedInAs)
      })
      .finally(() => setLoading(false))
  }, [reload])

  async function handleCreate(input: BlogInput) {
    await createBlog(input)
    setCreating(false)
    await reload(uid)
  }

  async function handleUpdate(id: string, input: BlogInput) {
    await updateBlog(id, input)
    setEditing(null)
    await reload(uid)
  }

  async function handleDelete(id: string) {
    await deleteBlog(id)
    await reload(uid)
  }

  if (loading) return <p>Loading blogs…</p>

  return (
    <div className="blogs">
      <h2>Blogs</h2>

      {error && <p role="alert">{error}</p>}

      {!uid && <p>Signed-in features are unavailable (Firebase isn't configured).</p>}

      {uid && !creating && <button onClick={() => setCreating(true)}>New blog</button>}
      {uid && creating && (
        <BlogForm onSubmit={handleCreate} onCancel={() => setCreating(false)} />
      )}

      <ul>
        {blogs.map((blog) => (
          <li key={blog.id}>
            {editing === blog.id ? (
              <BlogForm
                initial={blog}
                onSubmit={(input) => handleUpdate(blog.id, input)}
                onCancel={() => setEditing(null)}
              />
            ) : (
              <>
                <h3>{blog.title}</h3>
                <p>{blog.content}</p>
                <p>
                  <em>{blog.visibility}</em>
                </p>
                {uid === blog.ownerId && (
                  <>
                    <button onClick={() => setEditing(blog.id)}>Edit</button>
                    <button onClick={() => handleDelete(blog.id)}>Delete</button>
                  </>
                )}
              </>
            )}
          </li>
        ))}
      </ul>
    </div>
  )
}
