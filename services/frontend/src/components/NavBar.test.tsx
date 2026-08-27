import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { renderWithApp } from '../testUtils'
import { NavBar } from './NavBar'

describe('NavBar', () => {
  it('renders the brand and new-post link', () => {
    renderWithApp(<NavBar />)
    expect(screen.getByRole('link', { name: 'Gorman Club' })).toHaveAttribute('href', '/')
    expect(screen.getByRole('link', { name: 'New post' })).toHaveAttribute('href', '/new')
  })

  it('toggles the theme when the icon button is clicked', async () => {
    const toggleTheme = vi.fn()
    renderWithApp(<NavBar />, { context: { toggleTheme } })

    await userEvent.click(screen.getByRole('button', { name: 'Toggle dark mode' }))
    expect(toggleTheme).toHaveBeenCalledTimes(1)
  })

  it("links the avatar to the signed-in user's own profile", () => {
    renderWithApp(<NavBar />, {
      context: { user: { id: 'uid-1', email: 'a@b.com', name: 'Ada' } },
    })

    expect(screen.getByRole('link', { name: 'Your profile' })).toHaveAttribute(
      'href',
      '/profile/uid-1',
    )
    expect(screen.getByText('A')).toBeInTheDocument()
  })

  it('renders the Google sign-in button host when signed out', () => {
    const renderSignInButton = vi.fn()
    renderWithApp(<NavBar />, { context: { renderSignInButton } })

    expect(renderSignInButton).toHaveBeenCalled()
  })
})
