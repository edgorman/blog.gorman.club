import { useEffect, useMemo, useRef, type ReactNode } from 'react'
import { useGoogleAuth } from '../hooks/useGoogleAuth'
import { createApi } from '../lib/api'
import { useTheme } from '../lib/theme'
import { AppContext, type AppContextValue } from './AppContext'

const BACKEND_URL = import.meta.env.VITE_BACKEND_URL

/** A short, stable stand-in for an author's name when it can't be resolved (see resolveAuthorName). */
function fallbackName(id: string): string {
  return id.length > 8 ? `${id.slice(0, 8)}…` : id
}

export function AppProvider({ children }: { children: ReactNode }) {
  const { user, authHeaders, error, ready, renderButton, signOut } = useGoogleAuth()
  const { theme, toggleTheme } = useTheme()

  const api = useMemo(
    () => (BACKEND_URL ? createApi(BACKEND_URL, authHeaders) : null),
    [authHeaders],
  )

  const nameCache = useRef(new Map<string, Promise<string>>())
  // A name resolved (or fallen back to) before the api was configured may now be resolvable.
  useEffect(() => {
    nameCache.current.clear()
  }, [api])

  const resolveAuthorName = useMemo(() => {
    return (id: string): Promise<string> => {
      const cached = nameCache.current.get(id)
      if (cached) return cached

      const lookup = !api
        ? Promise.resolve(fallbackName(id))
        : api.getUser(id).then(
            (found) => found.displayName || fallbackName(id),
            () => fallbackName(id),
          )

      nameCache.current.set(id, lookup)
      return lookup
    }
  }, [api])

  const value: AppContextValue = {
    user,
    api,
    authError: error,
    authReady: ready,
    renderSignInButton: renderButton,
    signOut,
    theme,
    toggleTheme,
    resolveAuthorName,
  }

  return <AppContext.Provider value={value}>{children}</AppContext.Provider>
}
