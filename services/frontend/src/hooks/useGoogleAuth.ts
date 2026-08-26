/**
 * Hook wrapping the Google Identity Services sign-in flow.
 *
 * There is no backend login call: the ID token Google returns is decoded client-side for display
 * only, and reused as the bearer credential on backend requests, which verify it per request.
 * Nothing here is trusted by the backend - the identity it acts on comes from re-verifying the
 * same token against Google's keys.
 *
 * Staying signed in across a refresh is handled in two layers, because Google's own automatic
 * sign-in is not guaranteed to fire:
 *
 *  1. The credential is cached in sessionStorage and restored on mount if it has not expired.
 *     This is the deterministic path - it does not depend on Google session state at all. It is
 *     sessionStorage rather than localStorage so the credential dies with the tab, keeping the
 *     window in which an XSS bug could read it as small as possible while still surviving a
 *     reload.
 *  2. When nothing valid was restored (new tab, or the cached credential aged out), prompt() asks
 *     Google to reissue one. With auto_select that is silent where Google allows it - which
 *     requires a single active Google session, prior consent, and under FedCM no more than one
 *     sign-in attempt in the preceding 10 minutes. Where it isn't allowed, One Tap appears and
 *     the rendered button stays as the explicit fallback.
 *
 * prompt() is what makes auto_select do anything: it is a One Tap setting, so initialising with
 * auto_select but only ever calling renderButton() leaves it inert.
 *
 * signOut clears the cache and calls disableAutoSelect(), so an explicit sign-out is not undone
 * by either layer on the next load.
 */
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { GoogleCredentialResponse } from '../types/google'

const SCRIPT_ELEMENT_ID = 'google-identity-services'
const AUTH_PROVIDER = 'google'

export interface GoogleUser {
  id: string
  email: string
  name: string
}

/** Where the credential is cached so a reload can restore it. */
const STORAGE_KEY = 'blog.gorman.club:google-credential'

interface CredentialPayload {
  sub: string
  email: string
  name?: string
  /** Expiry, in seconds since the epoch. Google always sets it; treated as required here. */
  exp?: number
}

function parseCredential(credential: string): CredentialPayload {
  // JWT payloads are base64url-encoded (`-`/`_`, no padding); atob() only understands standard
  // base64 (`+`/`/`), so translate before decoding.
  const base64 = credential.split('.')[1].replace(/-/g, '+').replace(/_/g, '/')
  // atob() returns a byte-per-character (Latin-1) string, which corrupts multi-byte UTF-8
  // characters (e.g. accented or non-Latin names) if parsed directly - decode the raw bytes as
  // UTF-8 instead.
  const bytes = Uint8Array.from(atob(base64), (c) => c.charCodeAt(0))
  return JSON.parse(new TextDecoder().decode(bytes)) as CredentialPayload
}

function toUser(payload: CredentialPayload): GoogleUser {
  return { id: payload.sub, email: payload.email, name: payload.name ?? payload.email }
}

function decodeCredential(credential: string): GoogleUser {
  return toUser(parseCredential(credential))
}

/** Exposed for tests: the decoding above is easy to get subtly wrong. */
export const decodeCredentialForTest = decodeCredential

/**
 * Restores a cached credential, or null when there is none, it has expired, or storage is
 * unavailable (Safari private mode throws rather than returning null). An expired credential is
 * dropped rather than returned: the backend would reject it, so restoring it would present a
 * signed-in UI whose every request 401s.
 */
function restoreCredential(): { credential: string; user: GoogleUser } | null {
  try {
    const credential = sessionStorage.getItem(STORAGE_KEY)
    if (!credential) return null

    const payload = parseCredential(credential)
    if (!payload.exp || payload.exp * 1000 <= Date.now()) {
      sessionStorage.removeItem(STORAGE_KEY)
      return null
    }
    return { credential, user: toUser(payload) }
  } catch {
    // A corrupt or unreadable entry is not worth keeping around.
    try {
      sessionStorage.removeItem(STORAGE_KEY)
    } catch {
      // Storage is unavailable entirely; nothing to clean up.
    }
    return null
  }
}

function persistCredential(credential: string) {
  try {
    sessionStorage.setItem(STORAGE_KEY, credential)
  } catch {
    // Storage unavailable - sign-in still works for this page view, it just won't survive a
    // reload, which is strictly better than failing the sign-in itself.
  }
}

function clearCredential() {
  try {
    sessionStorage.removeItem(STORAGE_KEY)
  } catch {
    // As above: nothing useful to do if storage is unavailable.
  }
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
  // Read per call rather than once at module scope, so tests can stub it per case; Vite inlines
  // the real value at build time either way, so this has no effect on production behaviour.
  const CLIENT_ID = import.meta.env.VITE_GOOGLE_CLIENT_ID

  // Read once, on mount, via a lazy initialiser: restoring during the first render means the UI
  // never flashes signed-out before flipping to signed-in.
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

      // Only when there is nothing to restore: a valid cached credential already signed the user
      // in, and prompting anyway would put One Tap in front of someone who is already signed in.
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
      size: 'large',
    })
  }, [])

  // There is no server-side session, so signing out is purely local: drop the credential and
  // stop Google from silently re-selecting the same account on the next page load, which is what
  // auto_select would otherwise do.
  const signOut = useCallback(() => {
    window.google?.accounts.id.disableAutoSelect()
    clearCredential()
    setUser(null)
    setToken(null)
  }, [])

  const authHeaders = useMemo<Record<string, string>>(() => {
    const headers: Record<string, string> = {}
    if (token) {
      headers.Authorization = `Bearer ${token}`
      headers['Authorization-Provider'] = AUTH_PROVIDER
    }
    return headers
  }, [token])

  return { user, authHeaders, error, ready, renderButton, signOut }
}
