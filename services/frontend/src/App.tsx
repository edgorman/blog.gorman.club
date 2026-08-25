import { useMemo } from 'react'
import { HealthCheck } from './components/HealthCheck'
import { SignIn } from './components/SignIn'
import { Profile } from './components/Profile'
import { Blogs } from './components/Blogs'
import { useGoogleAuth } from './hooks/useGoogleAuth'
import { createApi } from './lib/api'
import './App.css'

const BACKEND_URL = import.meta.env.VITE_BACKEND_URL

function App() {
  const { user, authHeaders, error, ready, renderButton, signOut } = useGoogleAuth()

  // Rebuilt when the credential changes, so requests always carry the current one. There is no
  // refresh: the credential lasts as long as the page does, and signing out clears it.
  const api = useMemo(
    () => (user && BACKEND_URL ? createApi(BACKEND_URL, authHeaders) : null),
    [user, authHeaders],
  )

  return (
    <main>
      <h1>blog.gorman.club</h1>
      <p>Engineering console for the backend API.</p>

      <HealthCheck />
      <SignIn
        user={user}
        error={error}
        ready={ready}
        renderButton={renderButton}
        signOut={signOut}
      />

      {api && user ? (
        <>
          <Profile api={api} uid={user.id} />
          <Blogs api={api} uid={user.id} />
        </>
      ) : (
        <section className="panel">
          <h2>Profile and blogs</h2>
          <p>
            {BACKEND_URL
              ? 'Sign in to call the API.'
              : 'No backend deployed yet - VITE_BACKEND_URL is unset.'}
          </p>
        </section>
      )}
    </main>
  )
}

export default App
