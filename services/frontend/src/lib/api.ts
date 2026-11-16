/**
 * Client for the backend API (see /services/backend). Callers pass the header map from
 * useGoogleAuth, so this module knows nothing about any auth provider.
 *
 * Types and routes come from `src/gen/schema.d.ts`, which is generated from the one contract in
 * `packages/api/openapi.yaml` (see packages/api/AGENTS.md). A path or method that contract does not
 * declare fails `tsc`, and CI fails a PR that adds a hand-written `export interface` here
 * (`root:wire-types`).
 */
import createClient from 'openapi-fetch'
import type { components, paths } from '../gen/schema'

type Schemas = components['schemas']

export type User = Schemas['User']
export type CurrentUser = Schemas['CurrentUser']
export type Blog = Schemas['Blog']
export type BlogPage = Schemas['BlogPage']
export type BlogRequest = Schemas['BlogRequest']
export type UpdateCurrentUserRequest = Schemas['UpdateCurrentUserRequest']
export type Comment = Schemas['Comment']
export type ReactionCount = Schemas['ReactionCount']
export type PageReactions = Schemas['PageReactions']
export type TargetReactions = Schemas['TargetReactions']
export type ChatMessage = Schemas['ChatMessage']
export type ChatEdit = Schemas['ChatEdit']
export type ChatRequest = Schemas['ChatRequest']
export type ChatReply = Schemas['ChatReply']
export type ChatHistory = Schemas['ChatHistory']
/** "public" or "private": what a post's `visibility` holds. */
export type Visibility = Schemas['Visibility']
export type ListBlogsParams = NonNullable<paths['/blogs']['get']['parameters']['query']>

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
 * Whether the signed-in caller wrote something attributed to username. Usernames are unique and a
 * post or comment resolves its author's on every read, so this is the "is it mine" check the wire
 * leaves a client - it is never sent a uid to compare (#253). Only for what to show: the backend
 * decides every write itself.
 */
export function isAuthor(profile: CurrentUser | null, username: string): boolean {
  return !!profile && !!username && profile.username === username
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

/** What a request carrying a credential the backend rejected tells its caller. */
export const SESSION_EXPIRED = 'Your sign-in has expired. Sign in again to carry on.'

export function createApi(baseUrl: string, authHeaders: AuthHeaders, onUnauthorized?: () => void) {
  const client = createClient<paths>({
    baseUrl,
    headers: authHeaders,
    // Looked up per call rather than captured, so a stubbed global fetch is honoured.
    fetch: (request) => globalThis.fetch(request),
  })

  // Every call below goes through this: it resolves with the body, or throws an ApiError for any
  // non-2xx response, ending the session on a refused credential.
  async function ok<T>(result: Promise<{ data?: T; error?: unknown; response: Response }>): Promise<T> {
    const { data, error, response } = await result

    // A 401 with a credential attached means the credential itself was refused - in practice, that
    // it expired - so the caller hears that rather than a raw status, and the session is ended.
    if (response.status === 401 && authHeaders.Authorization) {
      onUnauthorized?.()
      throw new ApiError(401, SESSION_EXPIRED)
    }

    if (!response.ok) {
      // Every non-2xx response is an ErrorResponse, but fall back to the status if it isn't one.
      const message =
        typeof error === 'object' && error !== null && 'error' in error && typeof error.error === 'string'
          ? error.error
          : undefined
      throw new ApiError(response.status, message ?? `Request failed with ${response.status}`)
    }

    return data as T
  }

  // A reaction write answers with the target's counts as they now stand; callers want the list.
  const reactions = (target: Promise<{ data?: TargetReactions; error?: unknown; response: Response }>) =>
    ok(target).then((counts) => counts.reactions)

  return {
    listBlogs: (params?: ListBlogsParams) => ok(client.GET('/blogs', { params: { query: params } })),
    // A post is addressed by its slug alone, since slugs are unique across every author:
    // "hello-world" names at most one post anywhere, and the second post under that title is
    // suffixed instead. The uid a post records its owner by is never a URL, exactly as for a
    // profile.
    getBlog: (slug: string) => ok(client.GET('/blogs/{slug}', { params: { path: { slug } } })),
    // The posts nearest in meaning to this one, already narrowed to what the caller may read, and
    // empty until the worker has embedded the post.
    getRelatedBlogs: (slug: string) =>
      ok(client.GET('/blogs/{slug}/related', { params: { path: { slug } } })).then((related) => related.posts),
    createBlog: (blog: BlogRequest) => ok(client.POST('/blogs', { body: blog })),
    updateBlog: (slug: string, blog: BlogRequest) =>
      ok(client.PUT('/blogs/{slug}', { params: { path: { slug } }, body: blog })),
    deleteBlog: (slug: string) => ok<void>(client.DELETE('/blogs/{slug}', { params: { path: { slug } } })),

    // A profile is addressed by its username; the Google sub it is keyed by is never a URL.
    getUser: (username: string) => ok(client.GET('/users/{username}', { params: { path: { username } } })),
    // The caller's own profile needs no name: the backend takes the owner from the credential,
    // which is also how a client discovers the username it was given at sign-up.
    getCurrentUser: () => ok(client.GET('/users/me')),
    putUser: (user: UpdateCurrentUserRequest) => ok(client.PUT('/users/me', { body: user })),
    deleteUser: () => ok<void>(client.DELETE('/users/me')),

    // Comments hang off their post like the chat below, but they are the readers' half of it: the
    // thread is readable by exactly whoever may read the post - signed out included, for a public
    // one - while writing to it needs a credential, since a comment is signed by whoever left it.
    listComments: (slug: string) =>
      ok(client.GET('/blogs/{slug}/comments', { params: { path: { slug } } })).then((thread) => thread.comments),
    createComment: (slug: string, body: string) =>
      ok(client.POST('/blogs/{slug}/comments', { params: { path: { slug } }, body: { body } })),
    // Deleting is allowed for the comment's author and for the post's owner, who moderates their
    // own post; the backend decides, and answers a 403 for anybody else.
    deleteComment: (slug: string, id: string) =>
      ok<void>(client.DELETE('/blogs/{slug}/comments/{id}', { params: { path: { slug, id } } })),
    // Approving overrules the classifier on a flagged comment, which only the post's owner may.
    approveComment: (slug: string, id: string) =>
      ok(client.PUT('/blogs/{slug}/comments/{id}/moderation', { params: { path: { slug, id } } })),

    // Reactions are read for the whole page at once and written one at a time. A write is
    // addressed rather than toggled - PUT puts the reaction there, DELETE takes it back - so a
    // retried click or a stale page lands where it was aiming instead of undoing itself. Both
    // answer with the target's counts as they now stand, since a bar is a shared number that this
    // client's own click cannot predict.
    getReactions: (slug: string) => ok(client.GET('/blogs/{slug}/reactions', { params: { path: { slug } } })),
    addReaction: (slug: string, emoji: string, commentId?: string) =>
      reactions(
        commentId === undefined
          ? client.PUT('/blogs/{slug}/reactions/{emoji}', { params: { path: { slug, emoji } } })
          : client.PUT('/blogs/{slug}/comments/{id}/reactions/{emoji}', {
              params: { path: { slug, id: commentId, emoji } },
            }),
      ),
    removeReaction: (slug: string, emoji: string, commentId?: string) =>
      reactions(
        commentId === undefined
          ? client.DELETE('/blogs/{slug}/reactions/{emoji}', { params: { path: { slug, emoji } } })
          : client.DELETE('/blogs/{slug}/comments/{id}/reactions/{emoji}', {
              params: { path: { slug, id: commentId, emoji } },
            }),
      ),

    // The assistant conversation hangs off the post it is about, since that is all a chat is: it
    // has no identity apart from its post. Every one of these requires the caller to own the post
    // and to have the assistant enabled.
    getChat: (slug: string) => ok(client.GET('/blogs/{slug}/chat', { params: { path: { slug } } })),
    sendChatMessage: (slug: string, body: ChatRequest) =>
      ok(client.POST('/blogs/{slug}/chat', { params: { path: { slug } }, body })),
    clearChat: (slug: string) => ok<void>(client.DELETE('/blogs/{slug}/chat', { params: { path: { slug } } })),
  }
}

export type Api = ReturnType<typeof createApi>
