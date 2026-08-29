import { useEffect, useState } from 'react'
import { Link, Navigate, useParams } from 'react-router-dom'
import { useApp } from '../context/AppContext'
import { ApiError, errorMessage, userPath } from '../lib/api'

/** Lets a signed-in user edit their own username and bio (PUT /users/me). */
export function EditProfile() {
  // The editor sits under the profile it edits, so the path names whose profile is being edited.
  // The write itself still goes to /users/me and takes its owner from the credential, so this is
  // only ever a claim about which profile the visitor meant - checked below against the one they
  // actually hold, never trusted as authority to edit it.
  const { username: routeUsername } = useParams<{ username: string }>()
  const { api, user, refreshProfile } = useApp()
  // The name as saved, which is where Cancel and the post-save redirect point. `draft` is what the
  // field holds, so an unsaved edit never changes where they go.
  const [username, setUsername] = useState<string | null>(null)
  const [draftUsername, setDraftUsername] = useState('')
  const [loading, setLoading] = useState(true)
  const [bio, setBio] = useState('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [loadError, setLoadError] = useState<string | null>(null)
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
      (e: unknown) => {
        // Only a 404 means "no profile yet", which leaves the form blank to be filled in. Any
        // other failure must not look like that: saving from a blank form would overwrite a real
        // bio with an empty one, so the form is withheld entirely.
        if (!(e instanceof ApiError && e.status === 404)) {
          setLoadError(errorMessage(e, 'Failed to load your profile'))
        }
        setLoading(false)
      },
    )
  }, [api, user])

  const save = () => {
    if (!api || !user) return
    setSaving(true)
    setError(null)
    api
      // An unchanged username is sent as undefined, which tells the backend to keep the one already
      // held - or, for a profile that has none yet, to assign one. Only an actual edit is sent, so
      // a blank form on a new profile asks for a generated name rather than for an empty one.
      .putUser({ username: draftUsername === (username ?? '') ? undefined : draftUsername, bio })
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

  if (loadError) {
    return (
      <div className="page">
        <p role="alert" className="center-note">
          {loadError}
        </p>
        <Link to="/">← Back to feed</Link>
      </div>
    )
  }

  // Checked before the mismatch guard below: a rename leaves the path naming the old username, and
  // a redirect to the new one must not be mistaken for someone editing a profile they don't hold.
  if (saved && username) return <Navigate to={userPath(username) ?? '/'} replace />

  // A visitor who follows another user's edit path is turned away here rather than by the backend,
  // which would have let the write through against their own profile - the path names a profile,
  // but /users/me decides whose is written, so the two have to be reconciled before the form is
  // shown. Lookups fold case server-side, so the comparison does too.
  if (!loading && username && routeUsername?.toLowerCase() !== username.toLowerCase()) {
    return (
      <div className="page">
        <p className="center-note">You can only edit your own profile.</p>
        <Link to={userPath(username) ?? '/'}>← Back to your profile</Link>
      </div>
    )
  }

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
            <Link to={(username && userPath(username)) || '/'} className="btn btn-secondary">
              Cancel
            </Link>
          </div>
        </>
      )}
    </div>
  )
}
