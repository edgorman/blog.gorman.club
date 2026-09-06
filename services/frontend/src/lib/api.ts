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
  /**
   * The topics the post is filed under, normalized server-side: lowercase, one hyphen between
   * words, so "Web Dev" and "web-dev" are one tag. Absent for an untagged post. They are how a
   * reader finds a post by subject rather than by date - `tagPath` builds the link - and say
   * nothing about who may read it, which is `visibility`'s alone.
   */
  tags?: string[]
  visibility: 'public' | 'private'
  allowedUserIds?: string[]
  createdAt: string
  updatedAt: string
}

/**
 * One page of `listBlogs`: the posts themselves, newest first, plus whether a further page
 * follows. There is no separate cursor - the `createdAt` on the last post here already is one,
 * fed back as `ListBlogsParams.startAfter` to continue.
 */
export interface BlogPage {
  posts: Blog[]
  hasMore: boolean
}

/** What `listBlogs` pages and scopes by - all optional, so the bare call still means "the feed". */
export interface ListBlogsParams {
  /** How many posts a page holds. The backend applies its own default and cap when omitted. */
  limit?: number
  /** Continues a previous page: the `createdAt` of the last post it held. */
  startAfter?: string
  /** Narrows to one author's posts - a profile feed's `User.id`, not their username. */
  ownerId?: string
  /** Narrows to one topic. Any spelling works - the backend normalizes it before matching. */
  tag?: string
  /**
   * Narrows to posts holding this term in their title or body, ignoring case. It is a plain
   * substring match rather than a search index, and it never widens what comes back: a post the
   * caller may not read stays hidden however exactly it is named.
   */
  q?: string
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
  /**
   * When this account's paid access runs out, and absent for one that has never subscribed. It is
   * what `assistantEnabled` above is decided from, which is why both are here rather than either
   * being derived from the other: this says when the entitlement lapses, that says whether this
   * deployment can honour it at all - a backend with no model configured enables nothing however
   * long somebody has paid for.
   *
   * A date in the past is a lapsed subscription rather than a live one, so it is compared against
   * the clock rather than merely tested for presence (see `lib/subscription`).
   */
  subscribedUntil?: string
}

/** Where to send a buyer to pay, from `createCheckout`. */
export interface CheckoutSession {
  /**
   * The payment provider's own hosted page. It is a URL to navigate to rather than a redirect the
   * browser has already followed: this app fetches the call, so a 303 would hand it the provider's
   * HTML instead of somewhere to go.
   */
  url: string
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

/**
 * One reader's comment on a post.
 *
 * A comment is never edited, only written and removed, so there is no `updatedAt` and no update
 * call below - a reply somebody has already read and answered cannot become a different reply.
 * `authorId` is carried for the same reason a post carries `ownerId`: it is how a client knows
 * whose comment it is looking at, and so whether to offer a delete button.
 */
export interface Comment {
  /** Assigned by the backend, and meaningful only beneath the post it hangs off. */
  id: string
  blogSlug: string
  authorId: string
  /**
   * The commenter's username, resolved server-side - the only handle a client holds for the
   * profile behind a comment, exactly as `Blog.authorUsername` is for a post.
   */
  authorUsername: string
  body: string
  createdAt: string
}

/**
 * One emoji on a post or a comment, as the bar draws it: the glyph, how many readers chose it, and
 * whether you are one of them. Who else reacted is deliberately not reported - a count and a "you
 * are in it" is the whole of what a bar shows, and naming the readers would make a one-click
 * gesture into a public record.
 */
export interface ReactionCount {
  emoji: string
  count: number
  reacted: boolean
}

/**
 * Every reaction on a post page: the post's own, and each reacted-to comment's by id. It is one
 * response because it is one query server-side - a comment's reactions are stored beneath the post
 * alongside the post's - so a page costs one request rather than one per comment.
 */
export interface PageReactions {
  post: ReactionCount[]
  comments: Record<string, ReactionCount[]>
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

    // Buying the subscription the assistant is gated on. It takes no arguments and names no
    // account on purpose: the purchase is for the caller the credential identifies and can be for
    // nobody else, exactly as /users/me writes nobody else's profile. What comes back is where to
    // send them; the subscription itself is written by the provider's webhook to the backend, so
    // a buyer returning to this app is not what grants anything - which is why the profile is
    // re-read on their return rather than assumed to have changed.
    createCheckout: () => request<CheckoutSession>(baseUrl, authHeaders, 'POST', '/billing/checkout'),
    // The provider's own page for a subscription that already exists: a card, an invoice history,
    // and the cancel button. It names no customer for the same reason the checkout names no
    // account - the backend reads it off the caller's own stored profile - and 404s for an account
    // that has never paid, which has nothing to manage.
    createPortalSession: () => request<CheckoutSession>(baseUrl, authHeaders, 'POST', '/billing/portal'),

    // Comments hang off their post like the chat below, but they are the readers' half of it: the
    // thread is readable by exactly whoever may read the post - signed out included, for a public
    // one - while writing to it needs a credential, since a comment is signed by whoever left it.
    listComments: (slug: string) =>
      request<Comment[]>(baseUrl, authHeaders, 'GET', commentsPath(slug)),
    createComment: (slug: string, body: string) =>
      request<Comment>(baseUrl, authHeaders, 'POST', commentsPath(slug), { body }),
    // Deleting is allowed for the comment's author and for the post's owner, who moderates their
    // own post; the backend decides, and answers a 403 for anybody else.
    deleteComment: (slug: string, id: string) =>
      request<void>(baseUrl, authHeaders, 'DELETE', `${commentsPath(slug)}/${encodeURIComponent(id)}`),

    // Reactions are read for the whole page at once and written one at a time. A write is
    // addressed rather than toggled - PUT puts the reaction there, DELETE takes it back - so a
    // retried click or a stale page lands where it was aiming instead of undoing itself. Both
    // answer with the target's counts as they now stand, since a bar is a shared number that this
    // client's own click cannot predict.
    getReactions: (slug: string) =>
      request<PageReactions>(baseUrl, authHeaders, 'GET', `${blogPath(slug)}/reactions`),
    addReaction: (slug: string, emoji: string, commentId?: string) =>
      request<ReactionCount[]>(baseUrl, authHeaders, 'PUT', reactionPath(slug, emoji, commentId)),
    removeReaction: (slug: string, emoji: string, commentId?: string) =>
      request<ReactionCount[]>(baseUrl, authHeaders, 'DELETE', reactionPath(slug, emoji, commentId)),

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
