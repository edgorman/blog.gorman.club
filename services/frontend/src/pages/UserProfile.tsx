import { useEffect, useState } from 'react'
import { Link, useParams, useSearchParams } from 'react-router-dom'
import { FeedList } from '../components/FeedList'
import { PageMeta } from '../components/PageMeta'
import { SubscriptionStatus } from '../components/SubscriptionStatus'
import { useApp } from '../context/AppContext'
import { errorMessage, userPath, type Blog } from '../lib/api'
import { formatDate } from '../lib/format'

interface ProfileInfo {
  /** What posts are fetched by - `listBlogs`' `ownerId` takes a uid, never a username. */
  id: string
  /** Taken from the fetched profile rather than the URL, so it carries the casing as stored. */
  username: string
  bio: string
  /**
   * Optional because the wire says so: `createdAt` is a `google.protobuf.Timestamp`, and protojson
   * may leave a message field out of the body, so the generated `User` types it as possibly
   * absent. The render below already guards on it.
   */
  memberSince?: string
}

type PostsState =
  | { phase: 'unconfigured' }
  | { phase: 'loading' }
  | { phase: 'error'; message: string }
  | { phase: 'ready'; posts: Blog[]; hasMore: boolean; loadingMore: boolean; loadMoreError?: string }

const FEED_SIZE = 10

/** How long after returning from Checkout the profile is read again, for a webhook that lands late. */
const CHECKOUT_REFETCH_MS = 5000

/** A single author's recent posts, with as much of their profile as the caller is allowed to see. */
export function UserProfile() {
  const { username } = useParams<{ username: string }>()
  const { api, profile: ownProfile, refreshProfile } = useApp()
  const [searchParams] = useSearchParams()
  // Stripe sends the browser back here after Checkout. Being redirected is not proof of payment -
  // only the webhook grants access - so this says the subscription is on its way and reads the
  // caller's profile again, now and once more shortly after.
  const returnedFromCheckout = searchParams.get('checkout') === 'success'
  const [profile, setProfile] = useState<ProfileInfo | null>(null)
  const [missing, setMissing] = useState(false)
  const [postsState, setPostsState] = useState<PostsState>(
    api ? { phase: 'loading' } : { phase: 'unconfigured' },
  )

  useEffect(() => {
    if (!api || !username) return
    // Cleared per username: the router reuses this component between profiles, so without it a
    // second profile would render the first one's header, or its posts, while the new one loads.
    setProfile(null)
    setMissing(false)
    setPostsState({ phase: 'loading' })

    let cancelled = false
    // An author who never set up a profile has no username, so nothing can address this page for
    // them - a lookup that misses means the name really is unclaimed.
    api.getUser(username).then(
      (u) => {
        if (!cancelled) setProfile({ id: u.id, username: u.username, bio: u.bio, memberSince: u.createdAt })
      },
      () => {
        if (!cancelled) setMissing(true)
      },
    )
    return () => {
      cancelled = true
    }
  }, [api, username])

  useEffect(() => {
    if (!returnedFromCheckout) return
    refreshProfile()
    const timer = setTimeout(refreshProfile, CHECKOUT_REFETCH_MS)
    return () => clearTimeout(timer)
  }, [returnedFromCheckout, refreshProfile])

  useEffect(() => {
    // Posts are fetched by the profile's uid, once it resolves, rather than filtered client-side
    // out of the whole feed - the point of scoping `listBlogs` by `ownerId` in the first place.
    if (!api || !profile) return
    setPostsState({ phase: 'loading' })

    let cancelled = false
    api
      .listBlogs({ ownerId: profile.id, limit: FEED_SIZE })
      .then((page) => {
        if (!cancelled) {
          setPostsState({ phase: 'ready', posts: page.posts, hasMore: page.hasMore, loadingMore: false })
        }
      })
      .catch((e: unknown) => {
        if (!cancelled) setPostsState({ phase: 'error', message: errorMessage(e, 'Failed to load posts') })
      })
    return () => {
      cancelled = true
    }
  }, [api, profile])

  const loadMore = () => {
    if (!api || !profile || postsState.phase !== 'ready' || postsState.loadingMore) return
    const cursor = postsState.posts.at(-1)?.createdAt
    setPostsState({ ...postsState, loadingMore: true, loadMoreError: undefined })

    api
      .listBlogs({ ownerId: profile.id, limit: FEED_SIZE, startAfter: cursor })
      .then((page) => {
        setPostsState((prev) =>
          prev.phase === 'ready'
            ? { phase: 'ready', posts: [...prev.posts, ...page.posts], hasMore: page.hasMore, loadingMore: false }
            : prev,
        )
      })
      .catch((e: unknown) => {
        setPostsState((prev) =>
          prev.phase === 'ready'
            ? { ...prev, loadingMore: false, loadMoreError: errorMessage(e, 'Failed to load more posts') }
            : prev,
        )
      })
  }

  const isOwn = profile !== null && ownProfile?.id === profile.id
  const subscribed = !!ownProfile?.subscribedUntil && new Date(ownProfile.subscribedUntil) > new Date()

  if (postsState.phase === 'unconfigured') {
    return (
      <div className="page">
        <p className="text-muted center-note">No backend deployed yet - no backend URL is configured.</p>
      </div>
    )
  }

  if (missing) {
    return (
      <div className="page">
        <p className="center-note">No such user.</p>
        <Link to="/">← Back to feed</Link>
      </div>
    )
  }

  return (
    <div className="page">
      {profile && <PageMeta title={profile.username} description={profile.bio || undefined} path={userPath(profile.username) ?? '/'} />}
      <header className="profile-header">
        <div className="profile-identity">
          <div className="profile-avatar">{(profile?.username ?? '?').charAt(0).toUpperCase()}</div>
          <div>
            <h1 className="title-profile">{profile?.username ?? 'Loading…'}</h1>
            {profile?.memberSince && (
              <span className="text-muted feed-row-date">
                Member since {formatDate(profile.memberSince)}
              </span>
            )}
          </div>
        </div>
        {profile?.bio && <p className="profile-bio text-muted">{profile.bio}</p>}
        {/* The subscription is the owner's business alone: it comes from their own /users/me, never
            from the public profile this page fetched, and only shows on their own page. */}
        {isOwn && (
          <>
            {returnedFromCheckout && !subscribed && (
              <p className="text-muted">Thanks! Your subscription will appear here shortly.</p>
            )}
            <SubscriptionStatus />
          </>
        )}
      </header>

      {postsState.phase === 'loading' && <p className="text-muted center-note">Loading…</p>}
      {postsState.phase === 'error' && (
        <p role="alert" className="center-note">
          {postsState.message}
        </p>
      )}
      {postsState.phase === 'ready' && postsState.posts.length === 0 && (
        <p className="text-muted center-note">No posts yet.</p>
      )}
      {postsState.phase === 'ready' && postsState.posts.length > 0 && (
        <>
          <FeedList posts={postsState.posts} />
          {(postsState.hasMore || postsState.loadMoreError) && (
            <div className="feed-load-more">
              {postsState.loadMoreError && <p role="alert">{postsState.loadMoreError}</p>}
              {postsState.hasMore && (
                <button
                  type="button"
                  className="btn btn-ghost"
                  onClick={loadMore}
                  disabled={postsState.loadingMore}
                >
                  {postsState.loadingMore ? 'Loading…' : 'Load more'}
                </button>
              )}
            </div>
          )}
        </>
      )}
    </div>
  )
}
