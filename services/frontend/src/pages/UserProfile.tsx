import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { FeedList } from '../components/FeedList'
import { useApp } from '../context/AppContext'
import { errorMessage, type Blog } from '../lib/api'
import { formatDate } from '../lib/format'

interface ProfileInfo {
  /** Taken from the fetched profile rather than the URL, so it carries the casing as stored. */
  username: string
  bio: string
  memberSince: string
}

type PostsState =
  | { phase: 'unconfigured' }
  | { phase: 'loading' }
  | { phase: 'error'; message: string }
  | { phase: 'ready'; posts: Blog[] }

const FEED_SIZE = 10

/** A single author's recent posts, with as much of their profile as the caller is allowed to see. */
export function UserProfile() {
  const { username } = useParams<{ username: string }>()
  const { api } = useApp()
  // Lookups fold case server-side, so a link may differ in case from the stored name. Posts are
  // matched against the folded name too, or /profile/ed-gorman would show Ed-Gorman's header with
  // none of their posts.
  const key = username?.toLowerCase()
  const [profile, setProfile] = useState<ProfileInfo | null>(null)
  const [missing, setMissing] = useState(false)
  const [postsState, setPostsState] = useState<PostsState>(
    api ? { phase: 'loading' } : { phase: 'unconfigured' },
  )

  useEffect(() => {
    if (!api || !username) return
    // Cleared per username: the router reuses this component between profiles, so without it a
    // second profile would render the first one's header, or its "No such user."
    setProfile(null)
    setMissing(false)

    let cancelled = false
    // An author who never set up a profile has no username, so nothing can address this page for
    // them - a lookup that misses means the name really is unclaimed.
    api.getUser(username).then(
      (u) => {
        if (!cancelled) setProfile({ username: u.username, bio: u.bio ?? '', memberSince: u.createdAt })
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
    if (!api || !key) return
    setPostsState({ phase: 'loading' })

    let cancelled = false
    api
      .listBlogs()
      .then((posts) => {
        // Matching on the author a post carries, rather than on the uid behind it, keeps this
        // independent of the profile lookup above - both run against the username at once.
        const byOwner = posts
          .filter((p) => p.authorUsername.toLowerCase() === key)
          .sort((a, b) => b.createdAt.localeCompare(a.createdAt))
          .slice(0, FEED_SIZE)
        if (!cancelled) setPostsState({ phase: 'ready', posts: byOwner })
      })
      .catch((e: unknown) => {
        if (!cancelled) setPostsState({ phase: 'error', message: errorMessage(e, 'Failed to load posts') })
      })
    return () => {
      cancelled = true
    }
  }, [api, key])

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
        <FeedList posts={postsState.posts} />
      )}
    </div>
  )
}
