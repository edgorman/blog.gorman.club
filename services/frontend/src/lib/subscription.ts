import type { CurrentUser } from './api'

/**
 * Whether the account's paid access is live right now.
 *
 * The date is compared against the clock rather than merely tested for presence, because a profile
 * keeps the expiry of a subscription that has run out: an account that paid last year still has a
 * `subscribedUntil`, and it grants nothing. This is the same question the backend answers for
 * itself before serving anything paid - what is shown here only has to agree with it, never to be
 * trusted in its place.
 */
export function isSubscribed(profile: Pick<CurrentUser, 'subscribedUntil'> | null): boolean {
  if (!profile?.subscribedUntil) return false
  return new Date(profile.subscribedUntil).getTime() > Date.now()
}
