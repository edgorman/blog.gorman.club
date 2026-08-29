import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { useGoogleAuth } from '../hooks/useGoogleAuth'
import { ApiError, createApi, type CurrentUser } from '../lib/api'
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

  const [profile, setProfile] = useState<CurrentUser | null>(null)
  // Bumped by refreshProfile to re-run the fetch below, so a rename is picked up without a reload.
  const [profileNonce, setProfileNonce] = useState(0)
  const refreshProfile = useCallback(() => setProfileNonce((n) => n + 1), [])

  useEffect(() => {
    if (!api || !user) {
      setProfile(null)
      return
    }

    let cancelled = false
    api
      .getCurrentUser()
      // A 404 means they are signed in but hold no profile yet - a new account, or one whose
      // profile was deleted. Creating it here is what gives every signed-in user a username
      // without waiting for them to visit the editor: the body is empty, so the backend assigns
      // the name. Anything else (offline, a 500) leaves profile null to be retried on reload.
      .catch((e: unknown) => {
        if (e instanceof ApiError && e.status === 404) return api.putUser({})
        throw e
      })
      .then(
        (found) => {
          if (!cancelled) setProfile(found)
        },
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
