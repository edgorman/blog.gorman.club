import { createContext, useContext } from 'react'
import type { GoogleUser } from '../hooks/useGoogleAuth'
import type { Api, User } from '../lib/api'
import type { Theme } from '../lib/theme'

export interface AppContextValue {
  user: GoogleUser | null
  api: Api | null
  authError: string | null
  authReady: boolean
  renderSignInButton: (element: HTMLElement) => void
  signOut: () => void
  theme: Theme
  toggleTheme: () => void
  /**
   * The signed-in caller's own profile (GET /users/me), or null when signed out, before it loads,
   * or when they have not set one up yet. It is how the app learns its own username: the Google
   * identity carries only a sub, which is no longer addressable.
   *
   * Post authors do not come from here - a post carries its own, since resolving one from ownerId
   * is exactly what the API no longer allows.
   */
  profile: User | null
  /** Re-reads `profile`, for after an edit that may have changed the username. */
  refreshProfile: () => void
}

// Exported so tests can render `<AppContext.Provider value={...}>` with a fake value instead of
// the real Google auth / fetch wiring AppProvider sets up.
export const AppContext = createContext<AppContextValue | null>(null)

export function useApp(): AppContextValue {
  const ctx = useContext(AppContext)
  if (!ctx) throw new Error('useApp must be used within AppProvider')
  return ctx
}
