import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { Api, CurrentUser } from '../lib/api'
import { renderWithApp } from '../testUtils'
import { AccountPanel } from './AccountPanel'

describe('AccountPanel', () => {
  it('renders neither a version link nor a staging badge when neither is set', () => {
    renderWithApp(<AccountPanel onClose={() => {}} />)
    expect(screen.queryByText('staging')).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /^v/ })).not.toBeInTheDocument()
  })

  // Only staging says so - see issue #152 - and the version is always a release tag, linked to
  // its GitHub Releases page.
  it('shows a staging badge and a version link to the GitHub release on staging', () => {
    renderWithApp(<AccountPanel onClose={() => {}} />, { context: { version: 'v1.2.3', environment: 'staging' } })
    expect(screen.getByText('staging')).toBeInTheDocument()
    const link = screen.getByRole('link', { name: 'v1.2.3' })
    expect(link).toHaveAttribute('href', 'https://github.com/edgorman/blog.gorman.club/releases/tag/v1.2.3')
    expect(link).toHaveAttribute('target', '_blank')
  })

  it('shows the version link but no staging badge on production', () => {
    renderWithApp(<AccountPanel onClose={() => {}} />, { context: { version: 'v1.4.0', environment: 'production' } })
    expect(screen.getByRole('link', { name: 'v1.4.0' })).toHaveAttribute(
      'href',
      'https://github.com/edgorman/blog.gorman.club/releases/tag/v1.4.0',
    )
    expect(screen.queryByText('staging')).not.toBeInTheDocument()
    expect(screen.queryByText('production')).not.toBeInTheDocument()
  })

  describe('subscription', () => {
    const googleUser = { id: 'uid-1', email: 'a@example.com', name: 'A' }
    const profile: CurrentUser = {
      id: 'uid-1',
      username: 'calm-smiling-kestrel',
      bio: '',
      assistantEnabled: false,
      billingEnabled: true,
    }

    it('shows when a live subscription runs out, with a way to manage it', () => {
      renderWithApp(<AccountPanel onClose={() => {}} />, {
        context: { user: googleUser, api: {} as Api, profile: { ...profile, subscribedUntil: '2099-03-01T00:00:00Z' } },
      })
      expect(screen.getByText('Subscribed until Mar 1, 2099')).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Manage subscription' })).toBeInTheDocument()
    })

    it('says when a subscription has ended and offers to subscribe again', () => {
      renderWithApp(<AccountPanel onClose={() => {}} />, {
        context: { user: googleUser, api: {} as Api, profile: { ...profile, subscribedUntil: '2020-03-01T00:00:00Z' } },
      })
      expect(screen.getByText('Subscription ended Mar 1, 2020')).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Subscribe' })).toBeInTheDocument()
    })

    it('offers no button where the deployment sells no subscription', () => {
      renderWithApp(<AccountPanel onClose={() => {}} />, {
        context: { user: googleUser, api: {} as Api, profile: { ...profile, billingEnabled: false } },
      })
      expect(screen.getByText('Not subscribed')).toBeInTheDocument()
      expect(screen.queryByRole('button', { name: 'Subscribe' })).not.toBeInTheDocument()
    })

    it('reports a checkout that could not be opened', async () => {
      const api = { createCheckout: jest.fn().mockRejectedValue(new Error('payment provider unavailable')) }
      renderWithApp(<AccountPanel onClose={() => {}} />, {
        context: { user: googleUser, api: api as unknown as Api, profile },
      })
      await userEvent.click(screen.getByRole('button', { name: 'Subscribe' }))
      await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('payment provider unavailable'))
      expect(api.createCheckout).toHaveBeenCalled()
    })
  })
})
