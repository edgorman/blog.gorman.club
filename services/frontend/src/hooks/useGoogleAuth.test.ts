import { act, renderHook } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { decodeCredentialForTest, useGoogleAuth } from './useGoogleAuth'

/** Builds a JWT-shaped string whose payload is base64url-encoded, as Google issues. */
function credential(payload: Record<string, unknown>): string {
  const json = JSON.stringify(payload)
  const base64url = btoa(String.fromCharCode(...new TextEncoder().encode(json)))
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=+$/, '')
  return `header.${base64url}.signature`
}

describe('decodeCredential', () => {
  it('reads the identity claims out of the payload', () => {
    const user = decodeCredentialForTest(
      credential({ sub: '1234567890', email: 'ed@example.com', name: 'Ed Gorman' }),
    )

    expect(user).toEqual({ id: '1234567890', email: 'ed@example.com', name: 'Ed Gorman' })
  })

  it('falls back to the email when the token carries no name', () => {
    const user = decodeCredentialForTest(credential({ sub: '1', email: 'ed@example.com' }))

    expect(user.name).toBe('ed@example.com')
  })

  // atob() is Latin-1, so a name outside ASCII is mangled unless the bytes are decoded as UTF-8.
  it('decodes multi-byte characters in a name correctly', () => {
    const user = decodeCredentialForTest(
      credential({ sub: '1', email: 'a@example.com', name: 'Ædwarð Gørman 東京' }),
    )

    expect(user.name).toBe('Ædwarð Gørman 東京')
  })

  // Google's base64url alphabet uses - and _ where standard base64 uses + and /.
  it('handles a payload whose base64url encoding contains - and _', () => {
    // This name encodes to a payload containing both substitution characters.
    const name = 'ÿÿÿ?>?>'
    const user = decodeCredentialForTest(credential({ sub: '1', email: 'a@b.c', name }))

    expect(user.name).toBe(name)
  })
})

const STORAGE_KEY = 'blog.gorman.club:google-credential'

/** A credential expiring the given number of seconds from now. */
function credentialExpiringIn(seconds: number): string {
  return credential({
    sub: '1',
    email: 'ed@example.com',
    name: 'Ed',
    exp: Math.floor(Date.now() / 1000) + seconds,
  })
}

function stubGoogle() {
  const id = {
    initialize: vi.fn(),
    renderButton: vi.fn(),
    prompt: vi.fn(),
    disableAutoSelect: vi.fn(),
  }
  window.google = { accounts: { id } }
  return id
}

describe('useGoogleAuth', () => {
  afterEach(() => {
    vi.unstubAllEnvs()
    sessionStorage.clear()
    delete window.google
  })

  // auto_select only applies to the One Tap flow, so initialising with it but never calling
  // prompt() leaves it inert - which is exactly the bug that made a refresh sign the user out.
  it('initialises with auto_select and prompts when there is nothing to restore', () => {
    vi.stubEnv('VITE_GOOGLE_CLIENT_ID', 'test-client-id')
    const id = stubGoogle()

    renderHook(() => useGoogleAuth())

    expect(id.initialize).toHaveBeenCalledWith(expect.objectContaining({ auto_select: true }))
    expect(id.prompt).toHaveBeenCalled()
  })

  // The deterministic half of staying signed in: the cached credential is what survives a reload,
  // rather than depending on Google choosing to reissue one.
  it('restores an unexpired cached credential on mount', () => {
    vi.stubEnv('VITE_GOOGLE_CLIENT_ID', 'test-client-id')
    sessionStorage.setItem(STORAGE_KEY, credentialExpiringIn(3600))
    const id = stubGoogle()

    const { result } = renderHook(() => useGoogleAuth())

    expect(result.current.user).toEqual({ id: '1', email: 'ed@example.com', name: 'Ed' })
    expect(result.current.authHeaders.Authorization).toMatch(/^Bearer /)
    // Already signed in, so One Tap must not be put in front of the user.
    expect(id.prompt).not.toHaveBeenCalled()
  })

  // Restoring an expired credential would render a signed-in UI whose every request 401s.
  it('discards an expired cached credential and prompts instead', () => {
    vi.stubEnv('VITE_GOOGLE_CLIENT_ID', 'test-client-id')
    sessionStorage.setItem(STORAGE_KEY, credentialExpiringIn(-60))
    const id = stubGoogle()

    const { result } = renderHook(() => useGoogleAuth())

    expect(result.current.user).toBeNull()
    expect(sessionStorage.getItem(STORAGE_KEY)).toBeNull()
    expect(id.prompt).toHaveBeenCalled()
  })

  // A credential with no exp can't be reasoned about, so it is treated as unusable.
  it('discards a cached credential with no expiry', () => {
    vi.stubEnv('VITE_GOOGLE_CLIENT_ID', 'test-client-id')
    sessionStorage.setItem(STORAGE_KEY, credential({ sub: '1', email: 'ed@example.com' }))
    stubGoogle()

    const { result } = renderHook(() => useGoogleAuth())

    expect(result.current.user).toBeNull()
  })

  // Garbage in storage must not take the whole app down on load.
  it('survives a corrupt cached credential', () => {
    vi.stubEnv('VITE_GOOGLE_CLIENT_ID', 'test-client-id')
    sessionStorage.setItem(STORAGE_KEY, 'not-a-jwt')
    stubGoogle()

    const { result } = renderHook(() => useGoogleAuth())

    expect(result.current.user).toBeNull()
    expect(sessionStorage.getItem(STORAGE_KEY)).toBeNull()
  })

  it('caches the credential Google hands back so the next load can restore it', () => {
    vi.stubEnv('VITE_GOOGLE_CLIENT_ID', 'test-client-id')
    const id = stubGoogle()
    const issued = credentialExpiringIn(3600)

    const { result } = renderHook(() => useGoogleAuth())
    const config = id.initialize.mock.calls[0][0] as {
      callback: (response: { credential: string }) => void
    }
    act(() => config.callback({ credential: issued }))

    expect(sessionStorage.getItem(STORAGE_KEY)).toBe(issued)
    expect(result.current.user?.email).toBe('ed@example.com')
  })

  // Sign-out has to clear both halves, or the next load would restore what was just signed out of.
  it('clears the cache and disables auto-select on sign out', () => {
    vi.stubEnv('VITE_GOOGLE_CLIENT_ID', 'test-client-id')
    sessionStorage.setItem(STORAGE_KEY, credentialExpiringIn(3600))
    const id = stubGoogle()

    const { result } = renderHook(() => useGoogleAuth())
    act(() => result.current.signOut())

    expect(id.disableAutoSelect).toHaveBeenCalled()
    expect(sessionStorage.getItem(STORAGE_KEY)).toBeNull()
    expect(result.current.user).toBeNull()
  })
})
