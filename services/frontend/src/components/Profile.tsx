import { useEffect, useState } from 'react'
import { ApiError, errorMessage, type Api, type User } from '../lib/api'

interface Props {
  api: Api
  uid: string
}

/** Exercises GET/PUT/DELETE /users/{id} against the signed-in caller's own profile. */
export function Profile({ api, uid }: Props) {
  const [displayName, setDisplayName] = useState('')
  const [bio, setBio] = useState('')
  const [profile, setProfile] = useState<User | null>(null)
  const [status, setStatus] = useState('Loading…')

  const load = () => {
    api
      .getUser(uid)
      .then((user) => {
        setProfile(user)
        setDisplayName(user.displayName)
        setBio(user.bio ?? '')
        setStatus('Loaded')
      })
      .catch((e: unknown) => {
        if (e instanceof ApiError && e.status === 404) {
          setProfile(null)
          setStatus('No profile yet - saving creates one (201).')
          return
        }
        setStatus(errorMessage(e, 'Failed to load profile'))
      })
  }

  useEffect(load, [api, uid])

  const save = () => {
    setStatus('Saving…')
    api
      .putUser(uid, { displayName, bio })
      .then((user) => {
        setProfile(user)
        setStatus('Saved')
      })
      .catch((e: unknown) => setStatus(errorMessage(e, 'Save failed')))
  }

  const remove = () => {
    setStatus('Deleting…')
    api
      .deleteUser(uid)
      .then(() => {
        setProfile(null)
        setDisplayName('')
        setBio('')
        setStatus('Deleted')
      })
      .catch((e: unknown) => setStatus(errorMessage(e, 'Delete failed')))
  }

  return (
    <section className="panel">
      <h2>Your profile</h2>
      <p className="status">{status}</p>

      <label>
        Display name
        <input value={displayName} onChange={(e) => setDisplayName(e.target.value)} />
      </label>
      <label>
        Bio
        <input value={bio} onChange={(e) => setBio(e.target.value)} />
      </label>

      <div className="actions">
        <button onClick={save} disabled={displayName.trim() === ''}>
          Save
        </button>
        <button onClick={remove} disabled={!profile}>
          Delete
        </button>
      </div>

      {profile && (
        <dl>
          <dt>createdAt</dt>
          <dd>{profile.createdAt}</dd>
          <dt>updatedAt</dt>
          <dd>{profile.updatedAt}</dd>
        </dl>
      )}
    </section>
  )
}
