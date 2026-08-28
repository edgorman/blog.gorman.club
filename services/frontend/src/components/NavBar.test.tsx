import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { renderWithApp } from '../testUtils'
import { NavBar } from './NavBar'

const author = { id: 'uid-1', email: 'a@b.com', name: 'Ada Lovelace' }
const profile = {
  id: 'uid-1',
  username: 'calm-smiling-kestrel',
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}

describe('NavBar', () => {
  it('renders the brand and theme toggle, with no account panel open by default', () => {
    renderWithApp(<NavBar />)
    expect(screen.getByRole('link', { name: 'blog, gorman club' })).toHaveAttribute('href', '/')
    expect(screen.getByRole('button', { name: 'Toggle dark mode' })).toBeInTheDocument()
    expect(screen.queryByRole('dialog', { name: 'Account' })).not.toBeInTheDocument()
  })

  it('toggles the theme when the icon button is clicked', async () => {
    const toggleTheme = vi.fn()
    renderWithApp(<NavBar />, { context: { toggleTheme } })

    await userEvent.click(screen.getByRole('button', { name: 'Toggle dark mode' }))
    expect(toggleTheme).toHaveBeenCalledTimes(1)
  })

  it('opens the account panel with New post, View profile, and Sign out when signed in', async () => {
    const signOut = vi.fn()
    renderWithApp(<NavBar />, { context: { user: author, profile, signOut } })

    await userEvent.click(screen.getByRole('button', { name: 'Account' }))
    const dialog = screen.getByRole('dialog', { name: 'Account' })
    expect(dialog).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'New post' })).toHaveAttribute('href', '/new')
    expect(screen.getByRole('link', { name: 'View profile' })).toHaveAttribute(
      'href',
      '/profile/calm-smiling-kestrel',
    )

    await userEvent.click(screen.getByRole('button', { name: 'Sign out' }))
    expect(signOut).toHaveBeenCalledTimes(1)
    expect(screen.queryByRole('dialog', { name: 'Account' })).not.toBeInTheDocument()
  })

  // The panel identifies you the way everyone else sees you. The Google account name is not that -
  // it is settable elsewhere and unrelated to how you are addressed here - so only the username
  // appears, with the email kept as a subtitle to say which account you signed in with.
  it('identifies the caller by username, with their email beneath it', async () => {
    renderWithApp(<NavBar />, { context: { user: author, profile } })

    await userEvent.click(screen.getByRole('button', { name: 'Account' }))

    expect(screen.getByText('calm-smiling-kestrel')).toBeInTheDocument()
    expect(screen.getByText('a@b.com')).toBeInTheDocument()
    expect(screen.queryByText('Ada Lovelace')).not.toBeInTheDocument()
  })

  // The button sits one tap from that panel, so it takes its initial from the same name rather
  // than from the Google one the panel no longer shows.
  it('takes the account button initial from the username', async () => {
    renderWithApp(<NavBar />, { context: { user: author, profile } })

    expect(screen.getByRole('button', { name: 'Account' })).toHaveTextContent('C')
  })

  it('offers the Google sign-in button in the panel when signed out', async () => {
    const renderSignInButton = vi.fn()
    renderWithApp(<NavBar />, { context: { renderSignInButton } })

    await userEvent.click(screen.getByRole('button', { name: 'Account' }))
    expect(screen.getByText('Sign in to publish and manage your posts.')).toBeInTheDocument()
    expect(renderSignInButton).toHaveBeenCalled()
  })

  it('closes the panel on backdrop click', async () => {
    renderWithApp(<NavBar />, { context: { user: author } })

    await userEvent.click(screen.getByRole('button', { name: 'Account' }))
    expect(screen.getByRole('dialog', { name: 'Account' })).toBeInTheDocument()

    // The backdrop is the dialog's positioning parent, not the dialog itself.
    const backdrop = screen.getByRole('dialog', { name: 'Account' }).parentElement!
    await userEvent.click(backdrop)
    expect(screen.queryByRole('dialog', { name: 'Account' })).not.toBeInTheDocument()
  })
})
