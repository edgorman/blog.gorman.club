import { auth } from './firebase'
import type { Blog } from './blogs'

export type BlogInput = Pick<Blog, 'title' | 'content' | 'visibility' | 'allowedUserIds'>

/**
 * Writes go through the backend API rather than the Firebase SDK directly, so createdAt/updatedAt
 * are set from a trustworthy server clock. Reads go straight through Firestore (see lib/blogs.ts).
 */
async function request<T>(path: string, init: RequestInit): Promise<T> {
  const backendUrl = import.meta.env.VITE_BACKEND_URL
  if (!backendUrl) throw new Error('VITE_BACKEND_URL is not configured')
  if (!auth?.currentUser) throw new Error('not signed in')

  const token = await auth.currentUser.getIdToken()
  const response = await fetch(`${backendUrl.replace(/\/$/, '')}${path}`, {
    ...init,
    headers: {
      ...init.headers,
      Authorization: `Bearer ${token}`,
    },
  })

  if (!response.ok) {
    const body = (await response.json().catch(() => null)) as { error?: string } | null
    throw new Error(body?.error ?? `request failed with ${response.status}`)
  }

  if (response.status === 204) return undefined as T

  return response.json() as Promise<T>
}

export function createBlog(input: BlogInput): Promise<Blog> {
  return request<Blog>('/blogs', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  })
}

export function updateBlog(id: string, input: BlogInput): Promise<Blog> {
  return request<Blog>(`/blogs/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  })
}

export function deleteBlog(id: string): Promise<void> {
  return request<void>(`/blogs/${id}`, { method: 'DELETE' })
}
