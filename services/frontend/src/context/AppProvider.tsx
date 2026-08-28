import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { useGoogleAuth } from '../hooks/useGoogleAuth'
import { createApi, type User } from '../lib/api'
import { useTheme } from '../lib/theme'
import { AppContext, type AppContextValue } from './AppContext'

const BACKEND_URL = import.meta.env.VITE_BACKEND_URL

export function AppProvider({ children }: { children: ReactNode }) {
  const { user, authHeaders, error, ready, renderButton, signOut } = useGoogleAuth()
  const { theme, toggleTheme } = useTheme()

  const api = useMemo(
    () => (BACKEND_URL ? createApi(BACKEND_URL, authHeaders) : null),
    [authHeaders],
  )

  const [profile, setProfile] = useState<User | null>(null)
  // Bumped by refreshProfile to re-run the fetch below, so a rename is picked up without a reload.
  const [profileNonce, setProfileNonce] = useState(0)
  const refreshProfile = useCallback(() => setProfileNonce((n) => n + 1), [])

  useEffect(() => {
    if (!api || !user) {
      setProfile(null)
      return
    }

    let cancelled = false
    api.getCurrentUser().then(
      (found) => {
        if (!cancelled) setProfile(found)
      },
      // A 404 means they are signed in but have not created a profile yet, which is a real state
      // rather than an error: EditProfile is where they resolve it.
      () => {
        if (!cancelled) setProfile(null)
      },
    )
    return () => {
      cancelled = true
    }
  }, [api, user, profileNonce])

  const value: AppContextValue = {
    user,
    api,
    authError: error,
    authReady: ready,
    renderSignInButton: renderButton,
    signOut,
    theme,
    toggleTheme,
    profile,
    refreshProfile,
  }

  return <AppContext.Provider value={value}>{children}</AppContext.Provider>
}
