import { act, renderHook } from '@testing-library/react'
import { decodeCredential, useGoogleAuth } from './useGoogleAuth'

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
    const user = decodeCredential(
      credential({ sub: '1234567890', email: 'ed@example.com', name: 'Ed Gorman' }),
    )

    expect(user).toEqual({ id: '1234567890', email: 'ed@example.com', name: 'Ed Gorman' })
  })

  it('falls back to the email when the token carries no name', () => {
    const user = decodeCredential(credential({ sub: '1', email: 'ed@example.com' }))

    expect(user.name).toBe('ed@example.com')
  })

  // atob() is Latin-1, so a name outside ASCII is mangled unless the bytes are decoded as UTF-8.
  it('decodes multi-byte characters in a name correctly', () => {
    const user = decodeCredential(
      credential({ sub: '1', email: 'a@example.com', name: 'Ædwarð Gørman 東京' }),
    )

    expect(user.name).toBe('Ædwarð Gørman 東京')
  })

  // Google's base64url alphabet uses - and _ where standard base64 uses + and /.
  it('handles a payload whose base64url encoding contains - and _', () => {
    // This name encodes to a payload containing both substitution characters.
    const name = 'ÿÿÿ?>?>'
    const user = decodeCredential(credential({ sub: '1', email: 'a@b.c', name }))

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
    delete process.env.VITE_GOOGLE_CLIENT_ID
    sessionStorage.clear()
    delete window.google
  })

  // auto_select only applies to the One Tap flow, so initialising with it but never calling
  // prompt() leaves it inert - which is exactly the bug that made a refresh sign the user out.
  it('initialises with auto_select and prompts when there is nothing to restore', () => {
    process.env.VITE_GOOGLE_CLIENT_ID = 'test-client-id'
    const id = stubGoogle()

    renderHook(() => useGoogleAuth())

    expect(id.initialize).toHaveBeenCalledWith(expect.objectContaining({ auto_select: true }))
    expect(id.prompt).toHaveBeenCalled()
  })

  // Without a width Google's placeholder fills the container and its iframe then shrinks to fit the
  // text, which is the jump seen when the account panel opens.
  it('renders the button at its container width, within Google\'s 200-400px range', () => {
    process.env.VITE_GOOGLE_CLIENT_ID = 'test-client-id'
    const id = stubGoogle()
    const { result } = renderHook(() => useGoogleAuth())

    const widthFor = (clientWidth: number) => {
      const element = document.createElement('div')
      Object.defineProperty(element, 'clientWidth', { value: clientWidth })
      result.current.renderButton(element)
      return id.renderButton.mock.lastCall![1].width
    }

    expect(widthFor(288)).toBe(288)
    expect(widthFor(0)).toBe(200)
    expect(widthFor(900)).toBe(400)
  })

  // The deterministic half of staying signed in: the cached credential is what survives a reload,
  // rather than depending on Google choosing to reissue one.
  it('restores an unexpired cached credential on mount', () => {
    process.env.VITE_GOOGLE_CLIENT_ID = 'test-client-id'
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
    process.env.VITE_GOOGLE_CLIENT_ID = 'test-client-id'
    sessionStorage.setItem(STORAGE_KEY, credentialExpiringIn(-60))
    const id = stubGoogle()

    const { result } = renderHook(() => useGoogleAuth())

    expect(result.current.user).toBeNull()
    expect(sessionStorage.getItem(STORAGE_KEY)).toBeNull()
    expect(id.prompt).toHaveBeenCalled()
  })

  // A credential with no exp can't be reasoned about, so it is treated as unusable.
  it('discards a cached credential with no expiry', () => {
    process.env.VITE_GOOGLE_CLIENT_ID = 'test-client-id'
    sessionStorage.setItem(STORAGE_KEY, credential({ sub: '1', email: 'ed@example.com' }))
    stubGoogle()

    const { result } = renderHook(() => useGoogleAuth())

    expect(result.current.user).toBeNull()
  })

  // Garbage in storage must not take the whole app down on load.
  it('survives a corrupt cached credential', () => {
    process.env.VITE_GOOGLE_CLIENT_ID = 'test-client-id'
    sessionStorage.setItem(STORAGE_KEY, 'not-a-jwt')
    stubGoogle()

    const { result } = renderHook(() => useGoogleAuth())

    expect(result.current.user).toBeNull()
    expect(sessionStorage.getItem(STORAGE_KEY)).toBeNull()
  })

  it('caches the credential Google hands back so the next load can restore it', () => {
    process.env.VITE_GOOGLE_CLIENT_ID = 'test-client-id'
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
    process.env.VITE_GOOGLE_CLIENT_ID = 'test-client-id'
    sessionStorage.setItem(STORAGE_KEY, credentialExpiringIn(3600))
    const id = stubGoogle()

    const { result } = renderHook(() => useGoogleAuth())
    act(() => result.current.signOut())

    expect(id.disableAutoSelect).toHaveBeenCalled()
    expect(sessionStorage.getItem(STORAGE_KEY)).toBeNull()
    expect(result.current.user).toBeNull()
  })

  // Nothing refreshes a credential, so an open tab must not stay signed in past its expiry.
  it('signs out and asks for a fresh credential when the restored one expires', () => {
    vi.useFakeTimers()
    try {
      process.env.VITE_GOOGLE_CLIENT_ID = 'test-client-id'
      sessionStorage.setItem(STORAGE_KEY, credentialExpiringIn(3600))
      const id = stubGoogle()

      const { result } = renderHook(() => useGoogleAuth())
      expect(result.current.user).not.toBeNull()

      act(() => vi.advanceTimersByTime(3600 * 1000))

      expect(result.current.user).toBeNull()
      expect(result.current.authHeaders).toEqual({})
      expect(sessionStorage.getItem(STORAGE_KEY)).toBeNull()
      // Not a sign-out: auto_select stays on so Google can reissue for the same account.
      expect(id.disableAutoSelect).not.toHaveBeenCalled()
      expect(id.prompt).toHaveBeenCalled()
    } finally {
      vi.useRealTimers()
    }
  })
})

