/**
 * Client for the backend API (see /services/backend). Every endpoint requires a Firebase ID
 * token as a bearer token; callers pass a getToken function so a fresh token is fetched per
 * request rather than a stale one being cached here.
 */

export interface Blog {
  id: string
  ownerId: string
  title: string
  content: string
  visibility: 'public' | 'private'
  allowedUserIds?: string[]
  createdAt: string
  updatedAt: string
}

export interface User {
  id: string
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

export type GetToken = () => Promise<string>

async function request<T>(
  baseUrl: string,
  getToken: GetToken,
  method: string,
  path: string,
  body?: unknown,
): Promise<T> {
  const response = await fetch(`${baseUrl.replace(/\/$/, '')}${path}`, {
    method,
    headers: {
      Authorization: `Bearer ${await getToken()}`,
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

export function createApi(baseUrl: string, getToken: GetToken) {
  return {
    listBlogs: () => request<Blog[]>(baseUrl, getToken, 'GET', '/blogs'),
    getBlog: (id: string) => request<Blog>(baseUrl, getToken, 'GET', `/blogs/${id}`),
    createBlog: (blog: Partial<Blog>) => request<Blog>(baseUrl, getToken, 'POST', '/blogs', blog),
    updateBlog: (id: string, blog: Partial<Blog>) =>
      request<Blog>(baseUrl, getToken, 'PUT', `/blogs/${id}`, blog),
    deleteBlog: (id: string) => request<void>(baseUrl, getToken, 'DELETE', `/blogs/${id}`),

    getUser: (id: string) => request<User>(baseUrl, getToken, 'GET', `/users/${id}`),
    putUser: (id: string, user: Partial<User>) =>
      request<User>(baseUrl, getToken, 'PUT', `/users/${id}`, user),
    deleteUser: (id: string) => request<void>(baseUrl, getToken, 'DELETE', `/users/${id}`),
  }
}

export type Api = ReturnType<typeof createApi>
