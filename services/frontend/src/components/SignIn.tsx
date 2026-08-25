import { useEffect, useState } from 'react'
import {
  GoogleAuthProvider,
  onAuthStateChanged,
  signInWithPopup,
  signOut,
  type User as AuthUser,
} from 'firebase/auth'
import { firebaseConfigured, getFirebaseAuth } from '../lib/firebase'

interface Props {
  user: AuthUser | null
  onUserChange: (user: AuthUser | null) => void
}

/** Sign-in panel. Firebase Auth is only used to mint the ID token the backend verifies. */
export function SignIn({ user, onUserChange }: Props) {
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!firebaseConfigured) return
    return onAuthStateChanged(getFirebaseAuth(), onUserChange)
  }, [onUserChange])

  if (!firebaseConfigured) {
    return (
      <section className="panel" data-state="unconfigured">
        <h2>Sign in</h2>
        <p>
          Firebase is not configured for this build. Set <code>VITE_FIREBASE_API_KEY</code>,{' '}
          <code>VITE_FIREBASE_AUTH_DOMAIN</code> and <code>VITE_FIREBASE_PROJECT_ID</code> (see
          the frontend README).
        </p>
      </section>
    )
  }

  const handleSignIn = () => {
    setError(null)
    signInWithPopup(getFirebaseAuth(), new GoogleAuthProvider()).catch((e: unknown) =>
      setError(e instanceof Error ? e.message : 'Sign-in failed'),
    )
  }

  return (
    <section className="panel">
      <h2>Sign in</h2>
      {user ? (
        <>
          <dl>
            <dt>uid</dt>
            <dd>
              <code>{user.uid}</code>
            </dd>
            <dt>email</dt>
            <dd>{user.email ?? '—'}</dd>
          </dl>
          <button onClick={() => void signOut(getFirebaseAuth())}>Sign out</button>
        </>
      ) : (
        <>
          <p>Signed out. The API rejects every request without a valid ID token.</p>
          <button onClick={handleSignIn}>Sign in with Google</button>
        </>
      )}
      {error && <p role="alert">{error}</p>}
    </section>
  )
}
