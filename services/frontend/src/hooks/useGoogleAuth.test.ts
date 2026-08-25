import { describe, expect, it } from 'vitest'
import { decodeCredentialForTest } from './useGoogleAuth'

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
