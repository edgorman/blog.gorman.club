import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import type { Api, User } from '../lib/api'
import { renderWithApp } from '../testUtils'
import { EditProfile } from './EditProfile'

const me = { id: 'uid-1', email: 'a@b.com', name: 'Ada' }

const profile: User = {
  id: 'uid-1',
  displayName: 'Ada Lovelace',
  bio: 'Writes things.',
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}

function fakeApi(overrides: Partial<Api> = {}): Api {
  return {
    listBlogs: vi.fn(),
    getBlog: vi.fn(),
    createBlog: vi.fn(),
    updateBlog: vi.fn(),
    deleteBlog: vi.fn(),
    getUser: vi.fn().mockResolvedValue(profile),
    putUser: vi.fn().mockResolvedValue(profile),
    deleteUser: vi.fn(),
    ...overrides,
  } as unknown as Api
}

describe('EditProfile', () => {
  it('pre-fills the form with the existing profile', async () => {
    renderWithApp(<EditProfile />, { context: { api: fakeApi(), user: me } })

    expect(await screen.findByDisplayValue('Ada Lovelace')).toBeInTheDocument()
    expect(screen.getByDisplayValue('Writes things.')).toBeInTheDocument()
  })

  it('falls back to the Google account name when no profile exists yet', async () => {
    const api = fakeApi({ getUser: vi.fn().mockRejectedValue(new Error('not found')) })
    renderWithApp(<EditProfile />, { context: { api, user: me } })

    expect(await screen.findByDisplayValue('Ada')).toBeInTheDocument()
  })

  it('saves edits via putUser', async () => {
    const api = fakeApi()
    renderWithApp(<EditProfile />, { context: { api, user: me } })

    const nameInput = await screen.findByDisplayValue('Ada Lovelace')
    await userEvent.clear(nameInput)
    await userEvent.type(nameInput, 'Ada L.')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    expect(api.putUser).toHaveBeenCalledWith(
      'uid-1',
      expect.objectContaining({ displayName: 'Ada L.', bio: 'Writes things.' }),
    )
  })

  it('prompts sign-in when signed out', () => {
    renderWithApp(<EditProfile />, { context: { api: fakeApi() } })

    expect(screen.getByText('Sign in to edit your profile.')).toBeInTheDocument()
  })
})
