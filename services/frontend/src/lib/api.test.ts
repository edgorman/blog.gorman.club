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
    const fetchMock = mockFetch({ json: () => Promise.resolve({ posts: [], hasMore: false }) })

    await createApi('https://api.example.com/', authHeaders).listBlogs()

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('https://api.example.com/blogs')
    expect(init.method).toBe('GET')
    expect((init.headers as Record<string, string>).Authorization).toBe('Bearer test-token')
    expect((init.headers as Record<string, string>)['Authorization-Provider']).toBe('google')
  })

  // Only the params a caller actually sets end up on the query string, so a bare `listBlogs()`
  // still reads as a plain `/blogs` request above.
  it('carries listBlogs params on the query string', async () => {
    const fetchMock = mockFetch({ json: () => Promise.resolve({ posts: [], hasMore: false }) })

    await createApi('https://api.example.com', authHeaders).listBlogs({
      limit: 10,
      startAfter: '2026-08-01T00:00:00Z',
      ownerId: 'uid-1',
      tag: 'web dev',
      q: 'firestore notes',
    })

    const [url] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe(
      'https://api.example.com/blogs?limit=10&startAfter=2026-08-01T00%3A00%3A00Z&ownerId=uid-1&tag=web+dev&q=firestore+notes',
    )
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
      createApi('https://api.example.com', authHeaders).deleteBlog('hello-world'),
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

  // A slug names one post across every author, so it is the whole of the path.
  it('fetches a single blog by its slug', async () => {
    const fetchMock = mockFetch({ json: () => Promise.resolve({ slug: 'hello-world' }) })

    await createApi('https://api.example.com', authHeaders).getBlog('hello-world')

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('https://api.example.com/blogs/hello-world')
    expect(init.method).toBe('GET')
  })

  // A comment is addressed beneath the post it is on, since it has no identity apart from it.
  // The checkout names no account: the purchase is for whoever the credential identifies, and
  // there is deliberately nothing in the request that could name anybody else.
  it('starts a checkout without naming an account', async () => {
    const fetchMock = mockFetch({ json: () => Promise.resolve({ url: 'https://checkout.test/c/pay/cs_1' }) })

    const session = await createApi('https://api.example.com', authHeaders).createCheckout()

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('https://api.example.com/billing/checkout')
    expect(init.method).toBe('POST')
    expect(init.body).toBeUndefined()
    expect(session.url).toBe('https://checkout.test/c/pay/cs_1')
  })

  // Managing an existing subscription names no customer either: the backend reads it off the
  // caller's own stored profile, so there is no id in the request to ask for somebody else's.
  it('opens the billing portal without naming a customer', async () => {
    const fetchMock = mockFetch({ json: () => Promise.resolve({ url: 'https://billing.test/p/session/1' }) })

    const session = await createApi('https://api.example.com', authHeaders).createPortalSession()

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('https://api.example.com/billing/portal')
    expect(init.method).toBe('POST')
    expect(init.body).toBeUndefined()
    expect(session.url).toBe('https://billing.test/p/session/1')
  })

  it('addresses comments beneath their post', async () => {
    const fetchMock = mockFetch({ status: 201, json: () => Promise.resolve({ id: 'cmt1' }) })
    const api = createApi('https://api.example.com', authHeaders)

    await api.listComments('hello-world')
    await api.createComment('hello-world', 'Nicely put.')
    await api.deleteComment('hello-world', 'cmt1')

    const calls = fetchMock.mock.calls as [string, RequestInit][]
    expect(calls[0][0]).toBe('https://api.example.com/blogs/hello-world/comments')
    expect(calls[0][1].method).toBe('GET')
    expect(calls[1][1].method).toBe('POST')
    expect(calls[1][1].body).toBe(JSON.stringify({ body: 'Nicely put.' }))
    expect(calls[2][0]).toBe('https://api.example.com/blogs/hello-world/comments/cmt1')
    expect(calls[2][1].method).toBe('DELETE')
  })

  // A reaction is addressed by what it is on and the emoji itself, and the emoji is escaped like
  // any other path segment - unlike a slug, it is not URL-safe by construction.
  it('addresses a reaction by its target and its emoji', async () => {
    const fetchMock = mockFetch({ json: () => Promise.resolve([]) })
    const api = createApi('https://api.example.com', authHeaders)

    await api.getReactions('hello-world')
    await api.addReaction('hello-world', '👍')
    await api.removeReaction('hello-world', '👍', 'cmt1')

    const calls = fetchMock.mock.calls as [string, RequestInit][]
    expect(calls[0][0]).toBe('https://api.example.com/blogs/hello-world/reactions')
    expect(calls[1][0]).toBe('https://api.example.com/blogs/hello-world/reactions/%F0%9F%91%8D')
    expect(calls[1][1].method).toBe('PUT')
    expect(calls[2][0]).toBe(
      'https://api.example.com/blogs/hello-world/comments/cmt1/reactions/%F0%9F%91%8D',
    )
    expect(calls[2][1].method).toBe('DELETE')
  })

  it('falls back to the status when the error body is not JSON', async () => {
    mockFetch({ ok: false, status: 502, json: () => Promise.reject(new Error('not json')) })

    const error = await createApi('https://api.example.com', authHeaders)
      .listBlogs()
      .catch((e: unknown) => e)

    expect((error as ApiError).message).toContain('502')
  })
})
