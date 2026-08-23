import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('./firebase', () => ({
  auth: { currentUser: null as { getIdToken: () => Promise<string> } | null },
}))

import { auth } from './firebase'
import { createBlog, deleteBlog, updateBlog } from './blogsApi'

const input = { title: 't', content: 'c', visibility: 'public' as const }

// The mocked module's `auth` is typed against firebase/auth's real Auth interface, where
// currentUser is read-only - cast to set it from tests.
function setCurrentUser(user: { getIdToken: () => Promise<string> } | null) {
  ;(auth as { currentUser: unknown }).currentUser = user
}

describe('blogsApi', () => {
  beforeEach(() => {
    vi.stubEnv('VITE_BACKEND_URL', 'https://backend.example.com')
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.unstubAllEnvs()
    setCurrentUser(null)
  })

  it('throws when not signed in', async () => {
    await expect(createBlog(input)).rejects.toThrow('not signed in')
  })

  it('throws when the backend URL is not configured', async () => {
    vi.stubEnv('VITE_BACKEND_URL', '')
    setCurrentUser({ getIdToken: () => Promise.resolve('token') })

    await expect(createBlog(input)).rejects.toThrow('VITE_BACKEND_URL')
  })

  it('sends a bearer token and returns the parsed body on success', async () => {
    setCurrentUser({ getIdToken: () => Promise.resolve('token-123') })
    const created = { id: '1', ownerId: 'u1', ...input, createdAt: '', updatedAt: '' }
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({ ok: true, status: 201, json: () => Promise.resolve(created) }),
    )

    await expect(createBlog(input)).resolves.toEqual(created)
    expect(fetch).toHaveBeenCalledWith(
      'https://backend.example.com/blogs',
      expect.objectContaining({
        method: 'POST',
        headers: expect.objectContaining({ Authorization: 'Bearer token-123' }),
      }),
    )
  })

  it('parses the backend error shape on failure', async () => {
    setCurrentUser({ getIdToken: () => Promise.resolve('token') })
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: false,
        status: 403,
        json: () => Promise.resolve({ error: 'forbidden' }),
      }),
    )

    await expect(updateBlog('1', input)).rejects.toThrow('forbidden')
  })

  it('resolves with no body on delete', async () => {
    setCurrentUser({ getIdToken: () => Promise.resolve('token') })
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, status: 204 }))

    await expect(deleteBlog('1')).resolves.toBeUndefined()
  })
})
