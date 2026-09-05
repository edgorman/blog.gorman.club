import { afterEach, describe, expect, it, vi } from 'vitest'
import type { CurrentUser } from './api'
import { isSubscribed } from './subscription'

const now = new Date('2026-04-01T12:00:00Z')

function profile(subscribedUntil?: string): CurrentUser {
  return {
    id: 'uid-1',
    username: 'calm-smiling-kestrel',
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
    assistantEnabled: false,
    ...(subscribedUntil ? { subscribedUntil } : {}),
  }
}

describe('isSubscribed', () => {
  afterEach(() => vi.useRealTimers())

  it('is true only while the expiry is still ahead', () => {
    vi.useFakeTimers().setSystemTime(now)

    expect(isSubscribed(profile('2026-05-01T00:00:00Z'))).toBe(true)
    // A profile keeps the expiry of a subscription that has run out, so the date has to be
    // compared against the clock rather than merely tested for presence.
    expect(isSubscribed(profile('2026-03-01T00:00:00Z'))).toBe(false)
  })

  // Never subscribed, no profile loaded yet, and signed out are all the same answer.
  it('is false when there is no expiry at all', () => {
    expect(isSubscribed(profile())).toBe(false)
    expect(isSubscribed(null)).toBe(false)
  })
})
