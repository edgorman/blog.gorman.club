import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { useGoogleAuth } from '../hooks/useGoogleAuth'
import { ApiError, createApi, type CurrentUser } from '../lib/api'
import { useTheme } from '../lib/theme'
import { AppContext, type AppContextValue } from './AppContext'

export function AppProvider({
  children,
  // Resolved once at bootstrap by main.tsx (see lib/config.ts) before this ever renders. The
  // default is only for a caller that skips that bootstrap - tests, and vite.config.ts's test env
  // stands in for it there.
  backendUrl = import.meta.env.VITE_BACKEND_URL,
}: {
  children: ReactNode
  backendUrl?: string
}) {
  const { user, authHeaders, error, ready, renderButton, signOut } = useGoogleAuth()
  const { theme, toggleTheme } = useTheme()

  const api = useMemo(
    () => (backendUrl ? createApi(backendUrl, authHeaders) : null),
    [backendUrl, authHeaders],
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
