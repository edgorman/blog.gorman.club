import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError, type Api, type CurrentUser } from '../lib/api'
import { renderWithApp } from '../testUtils'
import { AccountPanel } from './AccountPanel'

const user = { id: 'uid-1', email: 'a@b.com', name: 'Ada Lovelace' }

function profile(overrides: Partial<CurrentUser> = {}): CurrentUser {
  return {
    id: 'uid-1',
    username: 'calm-smiling-kestrel',
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
    assistantEnabled: false,
    ...overrides,
  }
}

/** An Api with only the two billing calls this panel makes. */
function fakeApi(overrides: Partial<Api> = {}): Api {
  return {
    createCheckout: vi.fn().mockResolvedValue({ url: 'https://checkout.test/c/pay/cs_1' }),
    createPortalSession: vi.fn().mockResolvedValue({ url: 'https://billing.test/p/session/1' }),
    ...overrides,
  } as unknown as Api
}

describe('AccountPanel', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('shows an unsubscribed account what it could buy', () => {
    renderWithApp(<AccountPanel onClose={() => {}} />, { context: { user, profile: profile() } })

    expect(screen.getByText('Not subscribed')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Subscribe' })).toBeInTheDocument()
    // Nothing to manage: this account has never reached a checkout, and the backend would 404.
    expect(screen.queryByRole('button', { name: 'Manage subscription' })).not.toBeInTheDocument()
  })

  // An account that is already paying is offered nothing: the one thing it could buy is the thing
  // it has, and renewal is the payment provider's business rather than a button here.
  it('shows a subscribed account when its access renews, and offers no checkout', () => {
    vi.useFakeTimers().setSystemTime(new Date('2026-04-01T12:00:00Z'))
    try {
      renderWithApp(<AccountPanel onClose={() => {}} />, {
        context: { user, profile: profile({ subscribedUntil: '2026-05-01T00:00:00Z' }) },
      })

      expect(screen.getByText('Subscribed')).toBeInTheDocument()
      expect(screen.getByText(/Renews May 1, 2026/)).toBeInTheDocument()
      expect(screen.queryByRole('button', { name: 'Subscribe' })).not.toBeInTheDocument()
      // The one thing a paying account needs from here is the provider's page, which is where
      // cancelling lives.
      expect(screen.getByRole('button', { name: 'Manage subscription' })).toBeInTheDocument()
    } finally {
      vi.useRealTimers()
    }
  })

  // A subscription that ran out still shows its date: "Expired Mar 1" answers the question
  // somebody looking at this screen actually has.
  it('shows a lapsed subscription as expired, and offers the checkout again', () => {
    vi.useFakeTimers().setSystemTime(new Date('2026-04-01T12:00:00Z'))
    try {
      renderWithApp(<AccountPanel onClose={() => {}} />, {
        context: { user, profile: profile({ subscribedUntil: '2026-03-01T00:00:00Z' }) },
      })

      expect(screen.getByText('Not subscribed')).toBeInTheDocument()
      expect(screen.getByText(/Expired Mar 1, 2026/)).toBeInTheDocument()
      // Both: there is access to buy back, and there is billing history to look at.
      expect(screen.getByRole('button', { name: 'Subscribe' })).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Manage subscription' })).toBeInTheDocument()
    } finally {
      vi.useRealTimers()
    }
  })

  it('sends the buyer to the checkout the backend names', async () => {
    const assign = vi.fn()
    vi.stubGlobal('location', { ...window.location, assign })
    const api = fakeApi()

    renderWithApp(<AccountPanel onClose={() => {}} />, {
      context: { user, profile: profile(), api },
    })
    await userEvent.click(screen.getByRole('button', { name: 'Subscribe' }))

    expect(api.createCheckout).toHaveBeenCalledTimes(1)
    await waitFor(() => expect(assign).toHaveBeenCalledWith('https://checkout.test/c/pay/cs_1'))
  })

  // Cancelling is the reason this button exists, and it is on the provider's page rather than
  // here: what comes back from it is a webhook, which is the only thing that writes access.
  it('sends a subscriber to the provider to manage or cancel', async () => {
    const assign = vi.fn()
    vi.stubGlobal('location', { ...window.location, assign })
    const api = fakeApi()

    renderWithApp(<AccountPanel onClose={() => {}} />, {
      context: { user, profile: profile({ subscribedUntil: '2099-01-01T00:00:00Z' }), api },
    })
    await userEvent.click(screen.getByRole('button', { name: 'Manage subscription' }))

    expect(api.createPortalSession).toHaveBeenCalledTimes(1)
    await waitFor(() => expect(assign).toHaveBeenCalledWith('https://billing.test/p/session/1'))
  })

  // A deployment with no payment provider configured answers 503 here, and the buyer is told
  // rather than left looking at a button that did nothing.
  it('reports a checkout that could not be started', async () => {
    const createCheckout = vi.fn().mockRejectedValue(new ApiError(503, 'payments are not configured'))

    renderWithApp(<AccountPanel onClose={() => {}} />, {
      context: { user, profile: profile(), api: fakeApi({ createCheckout }) },
    })
    await userEvent.click(screen.getByRole('button', { name: 'Subscribe' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('payments are not configured')
    // Still offered, so a buyer can try again once the deployment is fixed.
    expect(screen.getByRole('button', { name: 'Subscribe' })).toBeEnabled()
  })

  // Nothing about a subscription is shown to somebody who is not signed in: there is no account
  // for it to be about.
  it('shows nothing about subscriptions when signed out', () => {
    renderWithApp(<AccountPanel onClose={() => {}} />)

    expect(screen.queryByText('Subscription')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Subscribe' })).not.toBeInTheDocument()
  })
})
