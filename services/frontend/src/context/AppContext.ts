import { createContext, useContext } from 'react'
import type { GoogleUser } from '../hooks/useGoogleAuth'
import type { Api } from '../lib/api'
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
   * Looks up a display name for a blog's ownerId. GET /users/{id} accepts any signed-in caller,
   * not just the owner, so this works for every author once someone is signed in; a signed-out
   * visitor falls back to a shortened id, since the backend has no unauthenticated user lookup.
   */
  resolveAuthorName: (id: string) => Promise<string>
}

// Exported so tests can render `<AppContext.Provider value={...}>` with a fake value instead of
// the real Google auth / fetch wiring AppProvider sets up.
export const AppContext = createContext<AppContextValue | null>(null)

export function useApp(): AppContextValue {
  const ctx = useContext(AppContext)
  if (!ctx) throw new Error('useApp must be used within AppProvider')
  return ctx
}
