/**
 * Client for the backend API (see /services/backend). Callers pass the header map from
 * useGoogleAuth, so this module knows nothing about any auth provider.
 */

/** The public half of a post's owner, resolved server-side. Null if they never set up a profile. */
export interface BlogAuthor {
  username: string
  displayName: string
}

export interface Blog {
  id: string
  ownerId: string
  author: BlogAuthor | null
  title: string
  content: string
  visibility: 'public' | 'private'
  allowedUserIds?: string[]
  createdAt: string
  updatedAt: string
}

export interface User {
  id: string
  username: string
  displayName: string
  bio?: string
  createdAt: string
  updatedAt: string
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

export function createApi(baseUrl: string, authHeaders: AuthHeaders) {
  return {
    listBlogs: () => request<Blog[]>(baseUrl, authHeaders, 'GET', '/blogs'),
    getBlog: (id: string) => request<Blog>(baseUrl, authHeaders, 'GET', `/blogs/${id}`),
    createBlog: (blog: Partial<Blog>) => request<Blog>(baseUrl, authHeaders, 'POST', '/blogs', blog),
    updateBlog: (id: string, blog: Partial<Blog>) =>
      request<Blog>(baseUrl, authHeaders, 'PUT', `/blogs/${id}`, blog),
    deleteBlog: (id: string) => request<void>(baseUrl, authHeaders, 'DELETE', `/blogs/${id}`),

    // A profile is addressed by its username; the Google sub it is keyed by is never a URL.
    getUser: (username: string) =>
      request<User>(baseUrl, authHeaders, 'GET', `/users/${encodeURIComponent(username)}`),
    // The caller's own profile needs no name: the backend takes the owner from the credential,
    // which is also how a client discovers the username it was given at sign-up.
    getCurrentUser: () => request<User>(baseUrl, authHeaders, 'GET', '/users/me'),
    putUser: (user: Partial<User>) => request<User>(baseUrl, authHeaders, 'PUT', '/users/me', user),
    deleteUser: () => request<void>(baseUrl, authHeaders, 'DELETE', '/users/me'),
  }
}

export type Api = ReturnType<typeof createApi>
