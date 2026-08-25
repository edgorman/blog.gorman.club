import { useMemo, useState } from 'react'
import type { User as AuthUser } from 'firebase/auth'
import { HealthCheck } from './components/HealthCheck'
import { SignIn } from './components/SignIn'
import { Profile } from './components/Profile'
import { Blogs } from './components/Blogs'
import { createApi } from './lib/api'
import './App.css'

const BACKEND_URL = import.meta.env.VITE_BACKEND_URL

function App() {
  const [user, setUser] = useState<AuthUser | null>(null)

  // Rebuilt whenever the signed-in user changes so the token getter always closes over the
  // current user; getIdToken() returns a fresh token, refreshing it when it has expired.
  const api = useMemo(
    () => (user && BACKEND_URL ? createApi(BACKEND_URL, () => user.getIdToken()) : null),
    [user],
  )

  return (
    <main>
      <h1>blog.gorman.club</h1>
      <p>Engineering console for the backend API.</p>

      <HealthCheck />
      <SignIn user={user} onUserChange={setUser} />

      {api && user ? (
        <>
          <Profile api={api} uid={user.uid} />
          <Blogs api={api} uid={user.uid} />
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
