/**
 * Client for the backend API (see /services/backend). Callers pass the header map from
 * useGoogleAuth, so this module knows nothing about any auth provider.
 *
 * Wire types come from `src/gen` rather than being declared here - see CLAUDE.md's "Contract
 * Layer". `User`/`CurrentUser` moved in #169, `Blog`/`BlogPage`/`ListBlogsParams` in #170,
 * `Comment`/`ReactionCount`/`PageReactions` in #171.
 */
import type { CurrentUser, User } from '../gen/blog/v1/user'
import type { Blog, BlogPage, ListBlogsParams } from '../gen/blog/v1/blog'
import type { Comment, CommentThread, CreateCommentRequest } from '../gen/blog/v1/comment'
import type { PageReactions, TargetReactions } from '../gen/blog/v1/reaction'

export type { User, CurrentUser } from '../gen/blog/v1/user'
export type { Blog, BlogPage, ListBlogsParams } from '../gen/blog/v1/blog'
export type { Comment } from '../gen/blog/v1/comment'
export type { ReactionCount, PageReactions, TargetReactions } from '../gen/blog/v1/reaction'

/**
 * "public" or "private" - what a post's `visibility` actually holds, kept as a union here rather
 * than derived from `Blog['visibility']`: the proto field is a plain string (see CLAUDE.md's
 * "Contract Layer" for why), so the generated `Blog` type carries no such constraint of its own.
 */
export type Visibility = 'public' | 'private'

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

/**
 * The feed filtered to one topic, which is where a tag chip links. Tags live in the feed's query
 * string rather than at a path of their own: a tag is one way of narrowing the feed, alongside the
 * search box beside it, rather than a page in its own right - so the two compose in the URL and
 * both survive a reload and a shared link.
 */
export function tagPath(tag: string): string {
  return `/?tag=${encodeURIComponent(tag)}`
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

/** The API path for a page of `GET /blogs`, carrying only the params a caller actually set. */
function blogsListPath(params: ListBlogsParams = {}): string {
  const query = new URLSearchParams()
  if (params.limit !== undefined) query.set('limit', String(params.limit))
  if (params.startAfter) query.set('startAfter', params.startAfter)
  if (params.ownerId) query.set('ownerId', params.ownerId)
  if (params.tag) query.set('tag', params.tag)
  if (params.q) query.set('q', params.q)

  const search = query.toString()
  return search ? `/blogs?${search}` : '/blogs'
}

/** The API path for one post, as `postPath` is its route in the console. */
function blogPath(slug: string): string {
  return `/blogs/${encodeURIComponent(slug)}`
}

/** The API path for one post's comments. */
function commentsPath(slug: string): string {
  return `${blogPath(slug)}/comments`
}

/**
 * The API path for one reaction: the thing reacted to, then the emoji. The emoji is escaped like
 * any other path segment - it is not URL-safe by construction the way a slug is.
 */
function reactionPath(slug: string, emoji: string, commentId?: string): string {
  const target = commentId === undefined ? blogPath(slug) : `${commentsPath(slug)}/${encodeURIComponent(commentId)}`
  return `${target}/reactions/${encodeURIComponent(emoji)}`
}

/** The API path for one post's assistant conversation. */
function chatPath(slug: string): string {
  return `${blogPath(slug)}/chat`
}

export function createApi(baseUrl: string, authHeaders: AuthHeaders) {
  return {
    listBlogs: (params?: ListBlogsParams) =>
      request<BlogPage>(baseUrl, authHeaders, 'GET', blogsListPath(params)),
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

    // Comments hang off their post like the chat below, but they are the readers' half of it: the
    // thread is readable by exactly whoever may read the post - signed out included, for a public
    // one - while writing to it needs a credential, since a comment is signed by whoever left it.
    //
    // `GET .../comments` answers with a `CommentThread` rather than a bare array - protojson can
    // only marshal a message at the top level (see CLAUDE.md's "Contract Layer") - so the list is
    // unwrapped here, and every other caller still sees a plain `Comment[]`.
    listComments: (slug: string) =>
      request<CommentThread>(baseUrl, authHeaders, 'GET', commentsPath(slug)).then((thread) => thread.comments),
    createComment: (slug: string, body: string) =>
      request<Comment>(baseUrl, authHeaders, 'POST', commentsPath(slug), { body } satisfies CreateCommentRequest),
    // Deleting is allowed for the comment's author and for the post's owner, who moderates their
    // own post; the backend decides, and answers a 403 for anybody else.
    deleteComment: (slug: string, id: string) =>
      request<void>(baseUrl, authHeaders, 'DELETE', `${commentsPath(slug)}/${encodeURIComponent(id)}`),

    // Reactions are read for the whole page at once and written one at a time. A write is
    // addressed rather than toggled - PUT puts the reaction there, DELETE takes it back - so a
    // retried click or a stale page lands where it was aiming instead of undoing itself. Both
    // answer with the target's counts as they now stand (wrapped in a `TargetReactions`, for the
    // same top-level-message reason `CommentThread` wraps a list above), since a bar is a shared
    // number that this client's own click cannot predict.
    getReactions: (slug: string) =>
      request<PageReactions>(baseUrl, authHeaders, 'GET', `${blogPath(slug)}/reactions`),
    addReaction: (slug: string, emoji: string, commentId?: string) =>
      request<TargetReactions>(baseUrl, authHeaders, 'PUT', reactionPath(slug, emoji, commentId)).then(
        (target) => target.reactions,
      ),
    removeReaction: (slug: string, emoji: string, commentId?: string) =>
      request<TargetReactions>(baseUrl, authHeaders, 'DELETE', reactionPath(slug, emoji, commentId)).then(
        (target) => target.reactions,
      ),

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
