import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { FeedList } from '../components/FeedList'
import { useApp } from '../context/AppContext'
import { errorMessage, type Blog } from '../lib/api'
import { formatDate } from '../lib/format'

interface ProfileInfo {
  name: string
  bio: string
  memberSince: string | null
  /** True once we know GET /users/{id} was reachable - vs. a signed-out fallback name. */
  resolved: boolean
}

type PostsState =
  | { phase: 'unconfigured' }
  | { phase: 'loading' }
  | { phase: 'error'; message: string }
  | { phase: 'ready'; posts: Blog[] }

const FEED_SIZE = 10

/** A single author's recent posts, with as much of their profile as the caller is allowed to see. */
export function UserProfile() {
  const { id } = useParams<{ id: string }>()
  const { api, resolveAuthorName } = useApp()
  const [profile, setProfile] = useState<ProfileInfo | null>(null)
  const [postsState, setPostsState] = useState<PostsState>(
    api ? { phase: 'loading' } : { phase: 'unconfigured' },
  )

  useEffect(() => {
    if (!api || !id) return
    api.getUser(id).then(
      (u) => setProfile({ name: u.displayName, bio: u.bio ?? '', memberSince: u.createdAt, resolved: true }),
      () => {
        resolveAuthorName(id).then((name) =>
          setProfile({ name, bio: '', memberSince: null, resolved: false }),
        )
      },
    )
  }, [api, id, resolveAuthorName])

  useEffect(() => {
    if (!api || !id) return
    setPostsState({ phase: 'loading' })
    api
      .listBlogs()
      .then((posts) => {
        const byOwner = posts
          .filter((p) => p.ownerId === id)
          .sort((a, b) => b.createdAt.localeCompare(a.createdAt))
          .slice(0, FEED_SIZE)
        setPostsState({ phase: 'ready', posts: byOwner })
      })
      .catch((e: unknown) => {
        setPostsState({ phase: 'error', message: errorMessage(e, 'Failed to load posts') })
      })
  }, [api, id])

  if (postsState.phase === 'unconfigured') {
    return (
      <div className="page">
        <p className="text-muted center-note">No backend deployed yet - VITE_BACKEND_URL is unset.</p>
      </div>
    )
  }

  return (
    <div className="page">
      <header className="profile-header">
        <div className="profile-identity">
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
        {profile && !profile.resolved && (
          <p className="profile-bio text-muted">Sign in to see this author's full profile.</p>
        )}
        {profile?.resolved && profile.bio && <p className="profile-bio text-muted">{profile.bio}</p>}
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
