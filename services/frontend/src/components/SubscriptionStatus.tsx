import type { CurrentUser } from '../lib/api'
import { formatDate } from '../lib/format'
import { isSubscribed } from '../lib/subscription'

interface Props {
  profile: CurrentUser
}

/**
 * What an account's subscription is worth to it right now: whether it is paying, and until when.
 *
 * It takes the caller's own profile rather than any profile, because only the caller's own carries
 * a `subscribedUntil` at all - a public profile lookup does not report who is paying, so there is
 * no way to render this for somebody else even by accident.
 *
 * A lapsed subscription still shows its date. "Expired Mar 3" answers the question somebody
 * looking at this screen actually has, where a bare "Not subscribed" would leave them wondering
 * whether the payment they remember making ever landed.
 */
export function SubscriptionStatus({ profile }: Props) {
  const subscribed = isSubscribed(profile)

  return (
    <p className="subscription-status">
      <span className={subscribed ? 'badge badge-active' : 'badge'}>
        {subscribed ? 'Subscribed' : 'Not subscribed'}
      </span>
      {profile.subscribedUntil && (
        <span className="text-muted subscription-date">
          {subscribed ? 'Renews' : 'Expired'} {formatDate(profile.subscribedUntil)}
        </span>
      )}
    </p>
  )
}
