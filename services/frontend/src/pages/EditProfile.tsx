import { useEffect, useState } from 'react'
import { Link, Navigate } from 'react-router-dom'
import { useApp } from '../context/AppContext'
import { errorMessage } from '../lib/api'

/** Lets a signed-in user edit their own display name and bio (PUT /users/{id}). */
export function EditProfile() {
  const { api, user } = useApp()
  const [loading, setLoading] = useState(true)
  const [displayName, setDisplayName] = useState('')
  const [bio, setBio] = useState('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [saved, setSaved] = useState(false)

  useEffect(() => {
    if (!api || !user) return
    api.getUser(user.id).then(
      (profile) => {
        setDisplayName(profile.displayName)
        setBio(profile.bio ?? '')
        setLoading(false)
      },
      () => {
        // No profile saved yet - prefill from the Google account name so there's something to edit.
        setDisplayName(user.name)
        setLoading(false)
      },
    )
  }, [api, user])

  const save = () => {
    if (!api || !user) return
    setSaving(true)
    setError(null)
    api
      .putUser(user.id, { displayName, bio })
      .then(() => setSaved(true))
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

  if (saved) return <Navigate to={`/profile/${user.id}`} replace />

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
            <label htmlFor="gc-display-name">Name</label>
            <input
              id="gc-display-name"
              className="input"
              placeholder="Your name"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
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
            <Link to={`/profile/${user.id}`} className="btn btn-secondary">
              Cancel
            </Link>
          </div>
        </>
      )}
    </div>
  )
}
