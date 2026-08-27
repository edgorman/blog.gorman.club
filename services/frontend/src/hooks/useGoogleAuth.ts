/**
 * Google Identity Services sign-in. There is no backend login call: the ID token Google returns is
 * decoded client-side for display only and reused as the bearer credential, which the backend
 * re-verifies against Google's keys per request.
 *
 * Staying signed in across a reload has two layers, because Google's automatic sign-in isn't
 * guaranteed to fire: the credential is cached in sessionStorage and restored on mount, and when
 * nothing valid was restored prompt() asks Google to reissue one (silently where it can, One Tap
 * otherwise, with the rendered button as the explicit fallback). prompt() is also what makes
 * auto_select apply, since it is a One Tap setting.
 */
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { GoogleCredentialResponse } from '../types/google'

const SCRIPT_ELEMENT_ID = 'google-identity-services'
const AUTH_PROVIDER = 'google'
const STORAGE_KEY = 'blog.gorman.club:google-credential'

export interface GoogleUser {
  id: string
  email: string
  name: string
}

interface CredentialPayload {
  sub: string
  email: string
  name?: string
  /** Expiry, in seconds since the epoch. Google always sets it; treated as required here. */
  exp?: number
}

function parseCredential(credential: string): CredentialPayload {
  // JWT payloads are base64url-encoded, which atob() doesn't understand, and atob() returns
  // Latin-1 bytes that corrupt multi-byte UTF-8 names unless decoded explicitly.
  const base64 = credential.split('.')[1].replace(/-/g, '+').replace(/_/g, '/')
  const bytes = Uint8Array.from(atob(base64), (c) => c.charCodeAt(0))
  return JSON.parse(new TextDecoder().decode(bytes)) as CredentialPayload
}

function toUser(payload: CredentialPayload): GoogleUser {
  return { id: payload.sub, email: payload.email, name: payload.name ?? payload.email }
}

/** Exported for tests: the base64url/UTF-8 decoding above is easy to get subtly wrong. */
export function decodeCredential(credential: string): GoogleUser {
  return toUser(parseCredential(credential))
}

/** Swallows the throw from storage being unavailable entirely (e.g. Safari private mode). */
function withStorage<T>(fn: () => T): T | undefined {
  try {
    return fn()
  } catch {
    return undefined
  }
}

/**
 * Restores a cached credential, or null when there is none, it has expired, or storage is
 * unavailable. An expired credential is dropped rather than returned, since restoring it would
 * present a signed-in UI whose every request 401s.
 */
function restoreCredential(): { credential: string; user: GoogleUser } | null {
  const restored = withStorage(() => {
    const credential = sessionStorage.getItem(STORAGE_KEY)
    if (!credential) return null

    const payload = parseCredential(credential)
    if (!payload.exp || payload.exp * 1000 <= Date.now()) return null
    return { credential, user: toUser(payload) }
  })

  if (!restored) clearCredential()
  return restored ?? null
}

function persistCredential(credential: string) {
  withStorage(() => sessionStorage.setItem(STORAGE_KEY, credential))
}

function clearCredential() {
  withStorage(() => sessionStorage.removeItem(STORAGE_KEY))
}

export interface UseGoogleAuthResult {
  user: GoogleUser | null
  authHeaders: Record<string, string>
  error: string | null
  ready: boolean
  renderButton: (element: HTMLElement) => void
  signOut: () => void
}

export function useGoogleAuth(): UseGoogleAuthResult {
  // Read per call rather than at module scope so tests can stub it; Vite inlines the real value
  // at build time either way.
  const CLIENT_ID = import.meta.env.VITE_GOOGLE_CLIENT_ID

  // Restoring in a lazy initialiser means the UI never flashes signed-out before signed-in.
  const [restored] = useState(restoreCredential)
  const [user, setUser] = useState<GoogleUser | null>(restored?.user ?? null)
  const [token, setToken] = useState<string | null>(restored?.credential ?? null)
  const [credentialError, setCredentialError] = useState<string | null>(null)
  const [ready, setReady] = useState(false)
  const initialized = useRef(false)

  // A missing client ID is known at render time, so it is derived rather than held in state.
  const error = CLIENT_ID
    ? credentialError
    : 'Google sign-in is not configured (missing VITE_GOOGLE_CLIENT_ID)'

  const handleCredentialResponse = useCallback((response: GoogleCredentialResponse) => {
    try {
      setUser(decodeCredential(response.credential))
      setToken(response.credential)
      setCredentialError(null)
      persistCredential(response.credential)
    } catch {
      setCredentialError('Failed to read the Google sign-in response')
    }
  }, [])

  useEffect(() => {
    if (!CLIENT_ID) return

    const initialize = () => {
      if (initialized.current || !window.google) return

      window.google.accounts.id.initialize({
        client_id: CLIENT_ID,
        callback: handleCredentialResponse,
        auto_select: true,
      })
      initialized.current = true
      setReady(true)

      // A restored credential already signed the user in, so prompting would put One Tap in front
      // of someone who is already signed in.
      if (!restored) {
        window.google.accounts.id.prompt()
      }
    }

    // The script tag is async, so it may or may not have finished loading by now.
    if (window.google) {
      initialize()
      return
    }

    const script = document.getElementById(SCRIPT_ELEMENT_ID)
    script?.addEventListener('load', initialize)
    return () => script?.removeEventListener('load', initialize)
  }, [CLIENT_ID, handleCredentialResponse, restored])

  const renderButton = useCallback((element: HTMLElement) => {
    if (!window.google || !initialized.current) return

    window.google.accounts.id.renderButton(element, {
      type: 'standard',
      theme: 'outline',
      size: 'medium',
    })
  }, [])

  // There is no server-side session, so signing out is purely local: drop the credential and stop
  // auto_select from silently re-selecting the same account on the next load.
  const signOut = useCallback(() => {
    window.google?.accounts.id.disableAutoSelect()
    clearCredential()
    setUser(null)
    setToken(null)
  }, [])

  const authHeaders = useMemo(
    (): Record<string, string> =>
      token ? { Authorization: `Bearer ${token}`, 'Authorization-Provider': AUTH_PROVIDER } : {},
    [token],
  )

  return { user, authHeaders, error, ready, renderButton, signOut }
}
