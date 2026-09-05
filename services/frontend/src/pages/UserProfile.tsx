import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { FeedList } from '../components/FeedList'
import { SubscriptionStatus } from '../components/SubscriptionStatus'
import { useApp } from '../context/AppContext'
import { errorMessage, type Blog } from '../lib/api'
import { formatDate } from '../lib/format'

interface ProfileInfo {
  /** What posts are fetched by - `listBlogs`' `ownerId` takes a uid, never a username. */
  id: string
  /** Taken from the fetched profile rather than the URL, so it carries the casing as stored. */
  username: string
  bio: string
  memberSince: string
}

type PostsState =
  | { phase: 'unconfigured' }
  | { phase: 'loading' }
  | { phase: 'error'; message: string }
  | { phase: 'ready'; posts: Blog[]; hasMore: boolean; loadingMore: boolean; loadMoreError?: string }

const FEED_SIZE = 10

/** A single author's recent posts, with as much of their profile as the caller is allowed to see. */
export function UserProfile() {
  const { username } = useParams<{ username: string }>()
  const { api, profile: me } = useApp()
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
        if (!cancelled) setProfile({ id: u.id, username: u.username, bio: u.bio ?? '', memberSince: u.createdAt })
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

  if (postsState.phase === 'unconfigured') {
    return (
      <div className="page">
        <p className="text-muted center-note">No backend deployed yet - VITE_BACKEND_URL is unset.</p>
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
        {/* Only on your own profile, and only from your own /users/me - a public lookup does not
            report who is paying, so this is not something that could be shown for anybody else
            even by mistake. The ids are compared rather than the usernames: a name is a handle its
            owner can change, and the uid is what both sides of this actually are. */}
        {me && profile && me.id === profile.id && <SubscriptionStatus profile={me} />}
        {profile?.bio && <p className="profile-bio text-muted">{profile.bio}</p>}
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
