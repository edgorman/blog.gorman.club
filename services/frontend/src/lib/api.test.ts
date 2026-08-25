import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError, createApi } from './api'

const authHeaders = { Authorization: 'Bearer test-token', 'Authorization-Provider': 'google' }

function mockFetch(response: Partial<Response>) {
  const fetchMock = vi.fn().mockResolvedValue({ ok: true, status: 200, ...response })
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}

afterEach(() => vi.unstubAllGlobals())

describe('createApi', () => {
  it('sends the auth headers and trims a trailing slash from the base URL', async () => {
    const fetchMock = mockFetch({ json: () => Promise.resolve([]) })

    await createApi('https://api.example.com/', authHeaders).listBlogs()

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('https://api.example.com/blogs')
    expect(init.method).toBe('GET')
    expect((init.headers as Record<string, string>).Authorization).toBe('Bearer test-token')
    expect((init.headers as Record<string, string>)['Authorization-Provider']).toBe('google')
  })

  it('serialises a JSON body for writes', async () => {
    const fetchMock = mockFetch({ status: 201, json: () => Promise.resolve({ id: 'blog-1' }) })

    await createApi('https://api.example.com', authHeaders).createBlog({
      title: 'Hello',
      visibility: 'public',
    })

    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect((init.headers as Record<string, string>)['Content-Type']).toBe('application/json')
    expect(JSON.parse(init.body as string)).toEqual({ title: 'Hello', visibility: 'public' })
  })

  // 204 has no body, so parsing it as JSON would throw.
  it('returns without parsing a body on 204', async () => {
    mockFetch({
      status: 204,
      json: () => Promise.reject(new Error('should not be called')),
    })

    await expect(
      createApi('https://api.example.com', authHeaders).deleteBlog('blog-1'),
    ).resolves.toBeUndefined()
  })

  // The status is what lets Profile treat "no profile yet" differently from a real failure.
  it('throws an ApiError carrying the status and the API error message', async () => {
    mockFetch({
      ok: false,
      status: 404,
      json: () => Promise.resolve({ error: 'user not found' }),
    })

    const error = await createApi('https://api.example.com', authHeaders)
      .getUser('missing')
      .catch((e: unknown) => e)

    expect(error).toBeInstanceOf(ApiError)
    expect((error as ApiError).status).toBe(404)
    expect((error as ApiError).message).toBe('user not found')
  })

  it('falls back to the status when the error body is not JSON', async () => {
    mockFetch({ ok: false, status: 502, json: () => Promise.reject(new Error('not json')) })

    const error = await createApi('https://api.example.com', authHeaders)
      .listBlogs()
      .catch((e: unknown) => e)

    expect((error as ApiError).message).toContain('502')
  })
})
