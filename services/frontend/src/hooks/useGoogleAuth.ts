/**
 * Hook wrapping the Google Identity Services sign-in flow.
 *
 * There is no backend login call: the ID token Google returns is decoded client-side for display
 * only, and reused as the bearer credential on backend requests, which verify it per request.
 * Nothing here is trusted by the backend - the identity it acts on comes from re-verifying the
 * same token against Google's keys.
 *
 * auto_select lets Google re-issue a credential on page load without a click, if the browser
 * still has a Google session and prior consent for this app - the credential itself still only
 * lives in memory, so a refresh re-requests it rather than reading anything back from storage.
 * signOut calls disableAutoSelect(), so an explicit sign-out is not silently undone by the next
 * page load.
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

function decodeCredential(credential: string): GoogleUser {
  // JWT payloads are base64url-encoded (`-`/`_`, no padding); atob() only understands standard
  // base64 (`+`/`/`), so translate before decoding.
  const base64 = credential.split('.')[1].replace(/-/g, '+').replace(/_/g, '/')
  // atob() returns a byte-per-character (Latin-1) string, which corrupts multi-byte UTF-8
  // characters (e.g. accented or non-Latin names) if parsed directly - decode the raw bytes as
  // UTF-8 instead.
  const bytes = Uint8Array.from(atob(base64), (c) => c.charCodeAt(0))
  const payload = JSON.parse(new TextDecoder().decode(bytes)) as {
    sub: string
    email: string
    name?: string
  }
  return { id: payload.sub, email: payload.email, name: payload.name ?? payload.email }
}

/** Exposed for tests: the decoding above is easy to get subtly wrong. */
export const decodeCredentialForTest = decodeCredential

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

  const [user, setUser] = useState<GoogleUser | null>(null)
  const [token, setToken] = useState<string | null>(null)
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
    }

    // The script tag is async, so it may or may not have finished loading by now.
    if (window.google) {
      initialize()
      return
    }

    const script = document.getElementById(SCRIPT_ELEMENT_ID)
    script?.addEventListener('load', initialize)
    return () => script?.removeEventListener('load', initialize)
  }, [CLIENT_ID, handleCredentialResponse])

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
