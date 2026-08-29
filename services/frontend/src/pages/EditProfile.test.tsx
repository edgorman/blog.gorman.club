import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { ApiError, type Api, type User } from '../lib/api'
import type { AppContextValue } from '../context/AppContext'
import { renderWithApp } from '../testUtils'
import { EditProfile } from './EditProfile'

const me = { id: 'uid-1', email: 'a@b.com', name: 'Ada' }

const profile: User = {
  id: 'uid-1',
  username: 'calm-smiling-kestrel',
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
    getUser: vi.fn(),
    getCurrentUser: vi.fn().mockResolvedValue(profile),
    putUser: vi.fn().mockResolvedValue(profile),
    deleteUser: vi.fn(),
    ...overrides,
  } as unknown as Api
}

/**
 * Renders the editor at the path it lives at: under the profile it edits. `at` names the profile
 * in the URL, which defaults to the caller's own - the mismatch case passes somebody else's.
 */
function renderEditor(context: Partial<AppContextValue>, at = 'calm-smiling-kestrel') {
  return renderWithApp(<EditProfile />, {
    context,
    route: `/user/${at}/edit`,
    path: '/user/:username/edit',
  })
}

describe('EditProfile', () => {
  it('pre-fills the form with the existing profile', async () => {
    renderEditor({ api: fakeApi(), user: me })

    expect(await screen.findByDisplayValue('calm-smiling-kestrel')).toBeInTheDocument()
    expect(screen.getByDisplayValue('Writes things.')).toBeInTheDocument()
  })

  // Nothing is prefilled before a profile exists: the username is assigned server-side on save.
  it('leaves the form empty when no profile exists yet', async () => {
    const api = fakeApi({
      getCurrentUser: vi.fn().mockRejectedValue(new ApiError(404, 'user not found')),
    })
    renderEditor({ api, user: me })

    expect(await screen.findByLabelText('Username')).toHaveValue('')
  })

  // Only a 404 means "no profile yet". Any other failure must withhold the form: saving from a
  // blank one would overwrite a real bio with an empty string.
  it('withholds the form when the profile cannot be loaded', async () => {
    const api = fakeApi({
      getCurrentUser: vi.fn().mockRejectedValue(new ApiError(500, 'internal error')),
    })
    renderEditor({ api, user: me })

    expect(await screen.findByRole('alert')).toHaveTextContent('internal error')
    expect(screen.queryByLabelText('Username')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Save' })).not.toBeInTheDocument()
  })

  // No id argument: the backend takes the owner from the credential.
  it('saves a renamed username via putUser', async () => {
    const api = fakeApi()
    renderEditor({ api, user: me })

    const nameInput = await screen.findByDisplayValue('calm-smiling-kestrel')
    await userEvent.clear(nameInput)
    await userEvent.type(nameInput, 'bold-leaping-lynx')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    expect(api.putUser).toHaveBeenCalledWith({
      username: 'bold-leaping-lynx',
      bio: 'Writes things.',
    })
  })

  // A profile that does not exist yet has no stored username, so a blank field is not an edit -
  // sending "" would ask for an empty name, which is rejected, rather than for a generated one.
  it('asks for a generated username when creating a profile', async () => {
    const api = fakeApi({
      getCurrentUser: vi.fn().mockRejectedValue(new ApiError(404, 'user not found')),
    })
    renderEditor({ api, user: me })

    const bioInput = await screen.findByLabelText('Bio')
    await userEvent.type(bioInput, 'Hello.')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    expect(api.putUser).toHaveBeenCalledWith({ username: undefined, bio: 'Hello.' })
  })

  // An untouched username is sent as undefined, so a bio-only edit is never read as a rename -
  // which would otherwise conflict with the name the profile already holds.
  it('omits an unchanged username', async () => {
    const api = fakeApi()
    renderEditor({ api, user: me })

    const bioInput = await screen.findByDisplayValue('Writes things.')
    await userEvent.clear(bioInput)
    await userEvent.type(bioInput, 'Writes other things.')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    expect(api.putUser).toHaveBeenCalledWith({
      username: undefined,
      bio: 'Writes other things.',
    })
  })

  // The path names a profile but /users/me decides whose is written, so a visitor following
  // somebody else's edit link has to be turned away here - the backend would have applied the
  // write to their own profile instead.
  it('refuses to edit a profile the caller does not hold', async () => {
    renderEditor({ api: fakeApi(), user: me }, 'bold-leaping-lynx')

    expect(await screen.findByText('You can only edit your own profile.')).toBeInTheDocument()
    expect(screen.queryByLabelText('Username')).not.toBeInTheDocument()
  })

  // Lookups fold case server-side, so a link that differs only in case is still the caller's own.
  it("edits the caller's own profile whatever the case in the path", async () => {
    renderEditor({ api: fakeApi(), user: me }, 'CALM-Smiling-Kestrel')

    expect(await screen.findByDisplayValue('calm-smiling-kestrel')).toBeInTheDocument()
  })

  it('prompts sign-in when signed out', () => {
    renderEditor({ api: fakeApi() })

    expect(screen.getByText('Sign in to edit your profile.')).toBeInTheDocument()
  })
})
