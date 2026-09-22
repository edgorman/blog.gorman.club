import { screen } from '@testing-library/react'
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
})
