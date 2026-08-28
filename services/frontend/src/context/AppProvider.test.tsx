import { render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError, type User } from '../lib/api'
import { useApp } from './AppContext'
import { AppProvider } from './AppProvider'

const getCurrentUser = vi.fn()
const putUser = vi.fn()

vi.mock('../hooks/useGoogleAuth', () => ({
  useGoogleAuth: () => ({
    user: { id: 'uid-1', email: 'a@b.com', name: 'Ada' },
    authHeaders: {},
    error: null,
    ready: true,
    renderButton: () => {},
    signOut: () => {},
  }),
}))

vi.mock('../lib/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib/api')>()),
  createApi: () => ({ getCurrentUser, putUser }),
}))

const profile: User = {
  id: 'uid-1',
  username: 'calm-smiling-kestrel',
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}

/** Surfaces the context's profile so a test can assert on it without rendering a page. */
function Probe() {
  const { profile: found } = useApp()
  return <span data-testid="username">{found?.username ?? 'none'}</span>
}

function renderProvider() {
  return render(
    <AppProvider>
      <Probe />
    </AppProvider>,
  )
}

describe('AppProvider', () => {
  beforeEach(() => {
    getCurrentUser.mockReset()
    putUser.mockReset()
  })

  it('uses the profile the caller already holds', async () => {
    getCurrentUser.mockResolvedValue(profile)
    renderProvider()

    await waitFor(() =>
      expect(screen.getByTestId('username')).toHaveTextContent('calm-smiling-kestrel'),
    )
    expect(putUser).not.toHaveBeenCalled()
  })

  // Signing in is the only "sign-up" the backend sees, so a caller with no profile gets one here
  // rather than having to visit the editor first - otherwise their posts have no author to show.
  it('creates a profile when the caller has none', async () => {
    getCurrentUser.mockRejectedValue(new ApiError(404, 'user not found'))
    putUser.mockResolvedValue(profile)
    renderProvider()

    await waitFor(() =>
      expect(screen.getByTestId('username')).toHaveTextContent('calm-smiling-kestrel'),
    )
    // An empty body: the username is the backend's to assign.
    expect(putUser).toHaveBeenCalledWith({})
  })

  // A failure that is not "no profile yet" must not be answered by creating one over the top of a
  // profile that may exist and simply could not be read.
  it('does not create a profile when the lookup fails for another reason', async () => {
    getCurrentUser.mockRejectedValue(new ApiError(500, 'internal error'))
    renderProvider()

    await waitFor(() => expect(screen.getByTestId('username')).toHaveTextContent('none'))
    expect(putUser).not.toHaveBeenCalled()
  })
})
