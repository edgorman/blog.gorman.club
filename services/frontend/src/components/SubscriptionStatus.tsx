import { useState } from 'react'
import { useApp } from '../context/AppContext'
import { errorMessage } from '../lib/api'
import { formatDate } from '../lib/format'

/**
 * The signed-in caller's own subscription: whether it is live and until when, with a button to
 * subscribe or to manage it. It reads only the caller's own profile (`GET /users/me`), so it can
 * only ever describe the account looking at it - UserProfile renders it on the owner's page alone.
 */
export function SubscriptionStatus() {
  const { api, profile } = useApp()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  if (!profile) return null

  const until = profile.subscribedUntil
  const live = until !== undefined && new Date(until) > new Date()

  const open = (session: () => Promise<string>) => {
    setBusy(true)
    setError(null)
    session().then(
      // Stripe hosts the payment pages, so this is a navigation away rather than anything embedded.
      (url) => window.location.assign(url),
      (e: unknown) => {
        setBusy(false)
        setError(errorMessage(e, 'Could not reach the payment provider'))
      },
    )
  }

  return (
    <div className="subscription-status">
      <p className="text-muted">
        {live
          ? `Subscribed until ${formatDate(until)}`
          : until
            ? `Subscription ended ${formatDate(until)}`
            : 'Not subscribed'}
      </p>
      {api && profile.billingEnabled && (
        <button
          type="button"
          className="btn btn-secondary btn-block"
          disabled={busy}
          onClick={() => open(live ? api.createPortal : api.createCheckout)}
        >
          {live ? 'Manage subscription' : 'Subscribe'}
        </button>
      )}
      {error && <p role="alert">{error}</p>}
    </div>
  )
}
