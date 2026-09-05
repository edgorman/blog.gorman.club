import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { useApp } from '../context/AppContext'
import { errorMessage, userPath, type CheckoutSession } from '../lib/api'
import { isSubscribed } from '../lib/subscription'
import { GoogleSignInButton } from './GoogleSignInButton'
import { SubscriptionStatus } from './SubscriptionStatus'

const CloseIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M18 6 6 18" />
    <path d="m6 6 12 12" />
  </svg>
)

interface Props {
  onClose: () => void
}

/**
 * The account overlay opened from NavBar's account button: sign in/out, a New post shortcut, and
 * what the account's subscription is worth to it.
 *
 * The subscription belongs here rather than only on the profile page because this is the panel
 * that answers "what does this account have?": it is where you find out you are signed in, and so
 * where you should find out that the assistant you cannot see is a thing you could buy.
 */
export function AccountPanel({ onClose }: Props) {
  const { api, user, profile, authError, authReady, renderSignInButton, signOut } = useApp()
  // The profile page and its editor are both addressed by the username, and the editor sits under
  // the profile it edits - so neither has a path until the profile has loaded.
  const profileHref = profile ? userPath(profile.username) : null
  const subscribed = isSubscribed(profile)
  // Held here rather than in the router: both billing destinations are places on the payment
  // provider, not routes of this app, so what this tracks is only the round trip that asks where
  // to send them.
  const [openingBilling, setOpeningBilling] = useState(false)
  const [billingError, setBillingError] = useState<string | null>(null)

  // Buying a subscription and managing one are the same gesture from here: ask the backend where
  // the provider wants this account sent, then go there. `start` returns nothing when there is no
  // backend configured at all, which is the same do-nothing this panel's other actions are.
  const openBilling = (start: () => Promise<CheckoutSession> | undefined, failure: string) => {
    if (openingBilling) return
    const opening = start()
    if (!opening) return

    setOpeningBilling(true)
    setBillingError(null)

    opening.then(
      (opened) => {
        // A full navigation rather than a router push: the destination is the provider's own page,
        // which this app neither owns nor renders.
        window.location.assign(opened.url)
      },
      (e: unknown) => {
        setOpeningBilling(false)
        setBillingError(errorMessage(e, failure))
      },
    )
  }

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', onKeyDown)
    return () => document.removeEventListener('keydown', onKeyDown)
  }, [onClose])

  return (
    <div className="panel-backdrop" onClick={onClose}>
      <div className="panel-drawer" role="dialog" aria-label="Account" onClick={(e) => e.stopPropagation()}>
        <div className="panel-drawer-header">
          <span className="panel-drawer-title">Account</span>
          <button type="button" className="btn btn-icon btn-secondary" aria-label="Close" onClick={onClose}>
            <CloseIcon />
          </button>
        </div>

        {user ? (
          <>
            <div className="panel-identity">
              <div className="gc-avatar" aria-hidden="true">
                {(profile?.username ?? '?').charAt(0).toUpperCase()}
              </div>
              <div>
                <div className="panel-identity-name">{profile?.username ?? 'Loading…'}</div>
                <div className="text-muted panel-identity-email">{user.email}</div>
              </div>
            </div>
            {profile && (
              <div className="panel-section">
                <span className="panel-section-title">Subscription</span>
                <SubscriptionStatus profile={profile} />
                {/* Only ever offered, never enforced here: the backend refuses a paid route to an
                    account that has not paid whatever this panel decided to draw.

                    A subscribed account is offered the provider's own page instead of another
                    checkout - it is where a card is changed, an invoice is read, and, the reason
                    it has to exist at all, where a subscription is cancelled. An account that has
                    paid before and lapsed sees both: it has billing to look at and access to buy
                    back. */}
                {!subscribed && (
                  <button
                    type="button"
                    className="btn btn-primary btn-block"
                    onClick={() => openBilling(() => api?.createCheckout(), 'Could not start checkout')}
                    disabled={openingBilling}
                  >
                    {openingBilling ? 'Opening…' : 'Subscribe'}
                  </button>
                )}
                {profile.subscribedUntil && (
                  <button
                    type="button"
                    className="btn btn-secondary btn-block"
                    onClick={() => openBilling(() => api?.createPortalSession(), 'Could not open the billing portal')}
                    disabled={openingBilling}
                  >
                    {openingBilling ? 'Opening…' : 'Manage subscription'}
                  </button>
                )}
                {billingError && <p role="alert">{billingError}</p>}
              </div>
            )}
            <Link to="/post/new" className="btn btn-primary btn-block" onClick={onClose}>
              New post
            </Link>
            {profileHref && (
              <>
                <Link to={profileHref} className="btn btn-secondary btn-block" onClick={onClose}>
                  View profile
                </Link>
                <Link to={`${profileHref}/edit`} className="btn btn-secondary btn-block" onClick={onClose}>
                  Edit profile
                </Link>
              </>
            )}
            <button type="button" className="btn btn-ghost btn-block" onClick={() => { signOut(); onClose() }}>
              Sign out
            </button>
          </>
        ) : (
          <>
            <p className="text-muted">Sign in to publish and manage your posts.</p>
            {authError ? <p role="alert">{authError}</p> : <GoogleSignInButton ready={authReady} onRender={renderSignInButton} />}
          </>
        )}
      </div>
    </div>
  )
}
