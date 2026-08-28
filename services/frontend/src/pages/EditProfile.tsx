import { useEffect, useState } from 'react'
import { Link, Navigate } from 'react-router-dom'
import { useApp } from '../context/AppContext'
import { errorMessage } from '../lib/api'

/** Lets a signed-in user edit their own username and bio (PUT /users/me). */
export function EditProfile() {
  const { api, user, refreshProfile } = useApp()
  // The name as saved, which is where Cancel and the post-save redirect point. `draft` is what the
  // field holds, so an unsaved edit never changes where they go.
  const [username, setUsername] = useState<string | null>(null)
  const [draftUsername, setDraftUsername] = useState('')
  const [loading, setLoading] = useState(true)
  const [bio, setBio] = useState('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [saved, setSaved] = useState(false)

  useEffect(() => {
    if (!api || !user) return
    api.getCurrentUser().then(
      (profile) => {
        setUsername(profile.username)
        setDraftUsername(profile.username)
        setBio(profile.bio ?? '')
        setLoading(false)
      },
      // No profile saved yet. Nothing is prefilled: saving assigns a username server-side, and
      // there is no other field the account could supply a sensible default for.
      () => setLoading(false),
    )
  }, [api, user])

  const save = () => {
    if (!api || !user) return
    setSaving(true)
    setError(null)
    api
      // An unchanged username is sent as undefined, which is what tells the backend to keep the one
      // already held rather than to treat it as a rename.
      .putUser({ username: draftUsername === username ? undefined : draftUsername, bio })
      .then((profile) => {
        setUsername(profile.username)
        setDraftUsername(profile.username)
        refreshProfile()
        setSaved(true)
      })
      .catch((e: unknown) => setError(errorMessage(e, 'Failed to save')))
      .finally(() => setSaving(false))
  }

  if (!api) {
    return (
      <div className="page">
        <p className="text-muted center-note">No backend deployed yet - VITE_BACKEND_URL is unset.</p>
      </div>
    )
  }

  if (!user) {
    return (
      <div className="page">
        <p className="center-note">Sign in to edit your profile.</p>
        <Link to="/">← Back to feed</Link>
      </div>
    )
  }

  if (saved && username) return <Navigate to={`/profile/${username}`} replace />

  return (
    <div className="page">
      <header className="page-header">
        <span className="page-kicker text-muted">Edit profile</span>
        <h1 className="title-editor">Your profile</h1>
      </header>

      {loading ? (
        <p className="text-muted center-note">Loading…</p>
      ) : (
        <>
          <div className="field">
            <label htmlFor="gc-username">Username</label>
            <input
              id="gc-username"
              className="input"
              placeholder="your-username"
              value={draftUsername}
              onChange={(e) => setDraftUsername(e.target.value)}
            />
          </div>

          <div className="field">
            <label htmlFor="gc-bio">Bio</label>
            <textarea
              id="gc-bio"
              className="input"
              placeholder="A short bio"
              rows={4}
              value={bio}
              onChange={(e) => setBio(e.target.value)}
            />
          </div>

          {error && (
            <p role="alert" style={{ marginTop: 'var(--space-3)' }}>
              {error}
            </p>
          )}

          <div style={{ display: 'flex', gap: 'var(--space-3)', marginTop: 'var(--space-4)' }}>
            <button type="button" className="btn btn-primary" onClick={save} disabled={saving}>
              {saving ? 'Saving…' : 'Save'}
            </button>
            <Link to={username ? `/profile/${username}` : '/'} className="btn btn-secondary">
              Cancel
            </Link>
          </div>
        </>
      )}
    </div>
  )
}
