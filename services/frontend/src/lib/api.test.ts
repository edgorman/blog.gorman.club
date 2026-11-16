import { ApiError, SESSION_EXPIRED, createApi, type BlogRequest } from './api'

const authHeaders = { Authorization: 'Bearer test-token', 'Authorization-Provider': 'google' }

const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })

function mockFetch(response: Response) {
  const fetchMock = vi.fn().mockResolvedValue(response)
  globalThis.fetch = fetchMock as unknown as typeof fetch
  return fetchMock
}

/** A fetch that answers every call with a fresh response, for a test that makes several. */
function mockFetchEach(make: () => Response) {
  const fetchMock = vi.fn().mockImplementation(() => Promise.resolve(make()))
  globalThis.fetch = fetchMock as unknown as typeof fetch
  return fetchMock
}

/** The Request the client handed to fetch. */
const sent = (fetchMock: ReturnType<typeof mockFetch>, call = 0) => fetchMock.mock.calls[call][0] as Request

const originalFetch = globalThis.fetch

afterEach(() => {
  globalThis.fetch = originalFetch
})

const blogRequest: BlogRequest = { title: 'Hello', content: '', tags: [], visibility: 'public', allowedUserIds: [] }

describe('createApi', () => {
  it('sends the auth headers and trims a trailing slash from the base URL', async () => {
    const fetchMock = mockFetch(json({ posts: [], hasMore: false }))

    await createApi('https://api.example.com/', authHeaders).listBlogs()

    const request = sent(fetchMock)
    expect(request.url).toBe('https://api.example.com/blogs')
    expect(request.method).toBe('GET')
    expect(request.headers.get('Authorization')).toBe('Bearer test-token')
    expect(request.headers.get('Authorization-Provider')).toBe('google')
  })

  // Only the params a caller actually sets end up on the query string, so a bare `listBlogs()`
  // still reads as a plain `/blogs` request above.
  it('carries listBlogs params on the query string', async () => {
    const fetchMock = mockFetch(json({ posts: [], hasMore: false }))

    await createApi('https://api.example.com', authHeaders).listBlogs({
      limit: 10,
      startAfter: '2026-08-01T00:00:00Z',
      author: 'calm-smiling-kestrel',
      tag: 'web dev',
      q: 'firestore notes',
    })

    expect(sent(fetchMock).url).toBe(
      'https://api.example.com/blogs?limit=10&startAfter=2026-08-01T00%3A00%3A00Z&author=calm-smiling-kestrel&tag=web%20dev&q=firestore%20notes',
    )
  })

  it('serialises a JSON body for writes', async () => {
    const fetchMock = mockFetch(json({ slug: 'hello' }, 201))

    await createApi('https://api.example.com', authHeaders).createBlog(blogRequest)

    const request = sent(fetchMock)
    expect(request.method).toBe('POST')
    expect(request.headers.get('Content-Type')).toBe('application/json')
    expect(await request.json()).toEqual(blogRequest)
  })

  // 204 has no body, so parsing it as JSON would throw.
  it('returns without parsing a body on 204', async () => {
    mockFetch(new Response(null, { status: 204 }))

    await expect(
      createApi('https://api.example.com', authHeaders).deleteBlog('hello-world'),
    ).resolves.toBeUndefined()
  })

  // The status is what lets Profile treat "no profile yet" differently from a real failure.
  it('throws an ApiError carrying the status and the API error message', async () => {
    mockFetch(json({ error: 'user not found' }, 404))

    const error = await createApi('https://api.example.com', authHeaders)
      .getUser('missing')
      .catch((e: unknown) => e)

    expect(error).toBeInstanceOf(ApiError)
    expect((error as ApiError).status).toBe(404)
    expect((error as ApiError).message).toBe('user not found')
  })

  // A slug names one post across every author, so it is the whole of the path.
  it('fetches a single blog by its slug', async () => {
    const fetchMock = mockFetch(json({ slug: 'hello-world' }))

    await createApi('https://api.example.com', authHeaders).getBlog('hello-world')

    expect(sent(fetchMock).url).toBe('https://api.example.com/blogs/hello-world')
    expect(sent(fetchMock).method).toBe('GET')
  })

  it('escapes a slug that arrives from the API', async () => {
    const fetchMock = mockFetch(json({ slug: 'a/b' }))

    await createApi('https://api.example.com', authHeaders).getBlog('a/b')

    expect(sent(fetchMock).url).toBe('https://api.example.com/blogs/a%2Fb')
  })

  // A comment is addressed beneath the post it is on, since it has no identity apart from it.
  it('addresses comments beneath their post', async () => {
    const fetchMock = mockFetchEach(() => json({ comments: [], id: 'cmt1' }))
    const api = createApi('https://api.example.com', authHeaders)

    await api.listComments('hello-world')
    await api.createComment('hello-world', 'Nicely put.')
    await api.approveComment('hello-world', 'cmt1')

    expect(sent(fetchMock, 0).url).toBe('https://api.example.com/blogs/hello-world/comments')
    expect(sent(fetchMock, 0).method).toBe('GET')
    expect(sent(fetchMock, 1).method).toBe('POST')
    expect(await sent(fetchMock, 1).text()).toBe(JSON.stringify({ body: 'Nicely put.' }))
    expect(sent(fetchMock, 2).url).toBe('https://api.example.com/blogs/hello-world/comments/cmt1/moderation')
    expect(sent(fetchMock, 2).method).toBe('PUT')
  })

  it('deletes a comment beneath its post', async () => {
    const fetchMock = mockFetch(new Response(null, { status: 204 }))

    await createApi('https://api.example.com', authHeaders).deleteComment('hello-world', 'cmt1')

    expect(sent(fetchMock).url).toBe('https://api.example.com/blogs/hello-world/comments/cmt1')
    expect(sent(fetchMock).method).toBe('DELETE')
  })

  // A reaction is addressed by what it is on and the emoji itself, and the emoji is escaped like
  // any other path segment - unlike a slug, it is not URL-safe by construction.
  it('addresses a reaction by its target and its emoji', async () => {
    const fetchMock = mockFetchEach(() => json({ reactions: [], post: [], comments: {} }))
    const api = createApi('https://api.example.com', authHeaders)

    await api.getReactions('hello-world')
    await expect(api.addReaction('hello-world', '👍')).resolves.toEqual([])
    await api.removeReaction('hello-world', '👍', 'cmt1')

    expect(sent(fetchMock, 0).url).toBe('https://api.example.com/blogs/hello-world/reactions')
    expect(sent(fetchMock, 1).url).toBe('https://api.example.com/blogs/hello-world/reactions/%F0%9F%91%8D')
    expect(sent(fetchMock, 1).method).toBe('PUT')
    expect(sent(fetchMock, 2).url).toBe(
      'https://api.example.com/blogs/hello-world/comments/cmt1/reactions/%F0%9F%91%8D',
    )
    expect(sent(fetchMock, 2).method).toBe('DELETE')
  })

  it('addresses the assistant conversation beneath its post', async () => {
    const fetchMock = mockFetchEach(() => json({ messages: [] }))
    const api = createApi('https://api.example.com', authHeaders)

    await api.getChat('hello-world')
    await api.sendChatMessage('hello-world', { message: 'shorter please' })

    expect(sent(fetchMock, 0).url).toBe('https://api.example.com/blogs/hello-world/chat')
    expect(sent(fetchMock, 0).method).toBe('GET')
    expect(sent(fetchMock, 1).method).toBe('POST')
    expect(await sent(fetchMock, 1).json()).toEqual({ message: 'shorter please' })
  })

  it('falls back to the status when the error body is not JSON', async () => {
    mockFetch(new Response('Bad Gateway', { status: 502 }))

    const error = await createApi('https://api.example.com', authHeaders)
      .listBlogs()
      .catch((e: unknown) => e)

    expect((error as ApiError).message).toContain('502')
  })

  it('ends the session and says so when the backend refuses the credential', async () => {
    mockFetch(json({ error: 'invalid token' }, 401))
    const onUnauthorized = vi.fn()

    const error = await createApi('https://api.example.com', authHeaders, onUnauthorized)
      .updateBlog('hello-world', blogRequest)
      .catch((e: unknown) => e)

    expect(onUnauthorized).toHaveBeenCalled()
    expect(error).toEqual(new ApiError(401, SESSION_EXPIRED))
  })

  it('leaves a signed-out 401 to the caller', async () => {
    mockFetch(json({ error: 'missing bearer token' }, 401))
    const onUnauthorized = vi.fn()

    const error = await createApi('https://api.example.com', {}, onUnauthorized)
      .createBlog(blogRequest)
      .catch((e: unknown) => e)

    expect(onUnauthorized).not.toHaveBeenCalled()
    expect((error as ApiError).message).toBe('missing bearer token')
  })
})
