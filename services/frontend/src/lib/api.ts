/**
 * Client for the backend API (see /services/backend). Callers pass the header map from
 * useGoogleAuth, so this module knows nothing about any auth provider.
 */

export interface Blog {
  /**
   * The whole of the post's address: the title, slugified, plus a short random suffix when some
   * post already holds that slug. Slugs are unique across every author, so a slug identifies one
   * post anywhere - use `postPath` to build a link. It is assigned when the post is created and
   * never changes, so a link keeps working after a retitle; render `title`, not this.
   */
  slug: string
  ownerId: string
  /**
   * The owner's username, resolved server-side. It is what an author is shown and linked as, not
   * part of the post's address. Empty only for a post whose owner holds no profile, which cannot
   * happen for one published since posting started naming its author - such a post is shown
   * unattributed.
   */
  authorUsername: string
  title: string
  content: string
  visibility: 'public' | 'private'
  allowedUserIds?: string[]
  createdAt: string
  updatedAt: string
}

export interface User {
  id: string
  /** The whole of a profile's public identity: both its address and the name readers see. */
  username: string
  bio?: string
  createdAt: string
  updatedAt: string
}

/**
 * The signed-in caller's own profile, which carries what this deployment lets that account do as
 * well as who they are. It is not what `getUser` returns: a public profile must not disclose who
 * has the assistant.
 */
export interface CurrentUser extends User {
  /**
   * Whether this account may use the AI writing assistant. The backend enforces it either way -
   * this only keeps the panel off the screen for somebody who would be told no.
   */
  assistantEnabled: boolean
}

/** One change the assistant made to the post, shown beneath the message that made it. */
export interface ChatEdit {
  tool: string
  summary: string
}

/** One turn of the conversation with the assistant. */
export interface ChatMessage {
  role: 'user' | 'assistant'
  content: string
  /** Absent for a user turn and for an assistant turn that only answered a question. */
  edits?: ChatEdit[]
  createdAt: string
}

/**
 * One exchange with the assistant: what was said on both sides, and the post as it now stands.
 *
 * The post comes back whole because the assistant edits the live draft server-side. When `updated`
 * is true the editor has to replace what is in its fields with `blog`, or the next save would
 * write the pre-assistant text back over it.
 */
export interface ChatReply {
  messages: ChatMessage[]
  blog: Blog
  updated: boolean
}

/** What the author is sending the assistant, along with the draft they are looking at. */
export interface ChatRequest {
  message: string
  /**
   * The unsaved draft on screen. Omitting these means "use the post as it was saved", which is why
   * they are sent even when unchanged: asking to tighten a paragraph has to mean the paragraph the
   * author can see.
   */
  title?: string
  content?: string
}

/** Thrown for any non-2xx response, carrying the status so callers can treat 404 as "absent". */
export class ApiError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

/** Message to show for a rejected request, falling back when it isn't an Error. */
export function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback
}

/**
 * The path a post lives at. The slug is escaped even though it is URL-safe by construction: it
 * arrives from the API, which is not this module's to trust.
 */
export function postPath(post: Pick<Blog, 'slug'>): string {
  return `/post/${encodeURIComponent(post.slug)}`
}

/**
 * The path a profile lives at, or null for an author with no username and so no page. Escaped for
 * the same reason `postPath` escapes a slug.
 */
export function userPath(username: string): string | null {
  if (!username) return null
  return `/user/${encodeURIComponent(username)}`
}

export type AuthHeaders = Record<string, string>

async function request<T>(
  baseUrl: string,
  authHeaders: AuthHeaders,
  method: string,
  path: string,
  body?: unknown,
): Promise<T> {
  const response = await fetch(`${baseUrl.replace(/\/$/, '')}${path}`, {
    method,
    headers: {
      ...authHeaders,
      ...(body === undefined ? {} : { 'Content-Type': 'application/json' }),
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  })

  if (!response.ok) {
    // Every non-2xx response is `{"error": "..."}`, but fall back to the status if that changes.
    const message = await response
      .json()
      .then((body: { error?: string }) => body.error)
      .catch(() => undefined)
    throw new ApiError(response.status, message ?? `Request failed with ${response.status}`)
  }

  if (response.status === 204) return undefined as T
  return response.json() as Promise<T>
}

/** The API path for one post, as `postPath` is its route in the console. */
function blogPath(slug: string): string {
  return `/blogs/${encodeURIComponent(slug)}`
}

/** The API path for one post's assistant conversation. */
function chatPath(slug: string): string {
  return `${blogPath(slug)}/chat`
}

export function createApi(baseUrl: string, authHeaders: AuthHeaders) {
  return {
    listBlogs: () => request<Blog[]>(baseUrl, authHeaders, 'GET', '/blogs'),
    // A post is addressed by its slug alone, since slugs are unique across every author:
    // "hello-world" names at most one post anywhere, and the second post under that title is
    // suffixed instead. The uid a post records its owner by is never a URL, exactly as for a
    // profile.
    getBlog: (slug: string) => request<Blog>(baseUrl, authHeaders, 'GET', blogPath(slug)),
    createBlog: (blog: Partial<Blog>) => request<Blog>(baseUrl, authHeaders, 'POST', '/blogs', blog),
    updateBlog: (slug: string, blog: Partial<Blog>) =>
      request<Blog>(baseUrl, authHeaders, 'PUT', blogPath(slug), blog),
    deleteBlog: (slug: string) => request<void>(baseUrl, authHeaders, 'DELETE', blogPath(slug)),

    // A profile is addressed by its username; the Google sub it is keyed by is never a URL.
    getUser: (username: string) =>
      request<User>(baseUrl, authHeaders, 'GET', `/users/${encodeURIComponent(username)}`),
    // The caller's own profile needs no name: the backend takes the owner from the credential,
    // which is also how a client discovers the username it was given at sign-up.
    getCurrentUser: () => request<CurrentUser>(baseUrl, authHeaders, 'GET', '/users/me'),
    putUser: (user: Partial<User>) =>
      request<CurrentUser>(baseUrl, authHeaders, 'PUT', '/users/me', user),
    deleteUser: () => request<void>(baseUrl, authHeaders, 'DELETE', '/users/me'),

    // The assistant conversation hangs off the post it is about, since that is all a chat is: it
    // has no identity apart from its post. Every one of these requires the caller to own the post
    // and to have the assistant enabled.
    getChat: (slug: string) =>
      request<{ messages: ChatMessage[] }>(baseUrl, authHeaders, 'GET', chatPath(slug)),
    sendChatMessage: (slug: string, body: ChatRequest) =>
      request<ChatReply>(baseUrl, authHeaders, 'POST', chatPath(slug), body),
    clearChat: (slug: string) => request<void>(baseUrl, authHeaders, 'DELETE', chatPath(slug)),
  }
}

export type Api = ReturnType<typeof createApi>
