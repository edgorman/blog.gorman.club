import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { Profile } from './Profile'
import { ApiError, type Api, type User } from '../lib/api'

const profile: User = {
  id: 'caller',
  displayName: 'Ed',
  bio: 'hello',
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-02T00:00:00Z',
}

function fakeApi(overrides: Partial<Api> = {}) {
  return {
    getUser: vi.fn().mockResolvedValue(profile),
    putUser: vi.fn().mockResolvedValue(profile),
    deleteUser: vi.fn().mockResolvedValue(undefined),
    ...overrides,
  } as unknown as Api
}

describe('Profile', () => {
  it('loads the existing profile into the form', async () => {
    render(<Profile api={fakeApi()} uid="caller" />)

    await waitFor(() => expect(screen.getByLabelText(/Display name/)).toHaveValue('Ed'))
    expect(screen.getByLabelText(/Bio/)).toHaveValue('hello')
  })

  // A missing profile is the normal first-run state, not an error to alarm anyone with.
  it('treats a 404 as "no profile yet" rather than a failure', async () => {
    const api = fakeApi({ getUser: vi.fn().mockRejectedValue(new ApiError(404, 'user not found')) })
    render(<Profile api={api} uid="caller" />)

    expect(await screen.findByText(/No profile yet/)).toBeInTheDocument()
  })

  it('reports a non-404 failure', async () => {
    const api = fakeApi({ getUser: vi.fn().mockRejectedValue(new ApiError(500, 'internal error')) })
    render(<Profile api={api} uid="caller" />)

    expect(await screen.findByText('internal error')).toBeInTheDocument()
  })

  it('saves the edited profile', async () => {
    const api = fakeApi()
    render(<Profile api={api} uid="caller" />)
    await waitFor(() => expect(screen.getByLabelText(/Display name/)).toHaveValue('Ed'))

    await userEvent.clear(screen.getByLabelText(/Display name/))
    await userEvent.type(screen.getByLabelText(/Display name/), 'Edward')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(api.putUser).toHaveBeenCalledWith('caller', { displayName: 'Edward', bio: 'hello' }),
    )
  })

  // The API rejects a blank displayName with 400, so don't let the request be made.
  it('disables Save while the display name is blank', async () => {
    const api = fakeApi({ getUser: vi.fn().mockRejectedValue(new ApiError(404, 'user not found')) })
    render(<Profile api={api} uid="caller" />)

    await screen.findByText(/No profile yet/)
    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled()
  })

  it('disables Delete when there is no profile to delete', async () => {
    const api = fakeApi({ getUser: vi.fn().mockRejectedValue(new ApiError(404, 'user not found')) })
    render(<Profile api={api} uid="caller" />)

    await screen.findByText(/No profile yet/)
    expect(screen.getByRole('button', { name: 'Delete' })).toBeDisabled()
  })
})
