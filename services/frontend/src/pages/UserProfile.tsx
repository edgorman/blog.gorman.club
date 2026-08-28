import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { FeedList } from '../components/FeedList'
import { useApp } from '../context/AppContext'
import { errorMessage, type Blog } from '../lib/api'
import { formatDate } from '../lib/format'

interface ProfileInfo {
  name: string
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
  const { api, profile: own } = useApp()
  const [profile, setProfile] = useState<ProfileInfo | null>(null)
  const [missing, setMissing] = useState(false)
  const [postsState, setPostsState] = useState<PostsState>(
    api ? { phase: 'loading' } : { phase: 'unconfigured' },
  )

  useEffect(() => {
    if (!api || !username) return
    // An author who never set up a profile has no username, so nothing can address this page for
    // them - a lookup that misses means the name really is unclaimed.
    api.getUser(username).then(
      (u) => setProfile({ name: u.displayName, bio: u.bio ?? '', memberSince: u.createdAt }),
      () => setMissing(true),
    )
  }, [api, username])

  useEffect(() => {
    if (!api || !username) return
    setPostsState({ phase: 'loading' })
    api
      .listBlogs()
      .then((posts) => {
        // Matching on the author a post carries, rather than on the uid behind it, keeps this
        // independent of the profile lookup above - both run against the username at once.
        const byOwner = posts
          .filter((p) => p.author?.username === username)
          .sort((a, b) => b.createdAt.localeCompare(a.createdAt))
          .slice(0, FEED_SIZE)
        setPostsState({ phase: 'ready', posts: byOwner })
      })
      .catch((e: unknown) => {
        setPostsState({ phase: 'error', message: errorMessage(e, 'Failed to load posts') })
      })
  }, [api, username])

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
        <div className="profile-identity" style={{ justifyContent: 'space-between' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-4)' }}>
            <div className="profile-avatar">{(profile?.name ?? '?').charAt(0).toUpperCase()}</div>
            <div>
              <h1 className="title-profile">{profile?.name ?? 'Loading…'}</h1>
              {profile?.memberSince && (
                <span className="text-muted feed-row-date">
                  Member since {formatDate(profile.memberSince)}
                </span>
              )}
            </div>
          </div>
          {own?.username === username && (
            <Link to="/profile/edit" className="btn btn-secondary">
              Edit profile
            </Link>
          )}
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
