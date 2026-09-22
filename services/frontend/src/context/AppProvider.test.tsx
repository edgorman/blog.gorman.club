import { render, screen, waitFor } from '@testing-library/react'
import { ApiError, type User } from '../lib/api'
import { useApp } from './AppContext'
import { AppProvider } from './AppProvider'

// Prefixed with "mock" so babel-plugin-jest-hoist hoists these declarations above the jest.mock()
// calls below along with the calls themselves - otherwise the factories would see them as TDZ.
const mockGetCurrentUser = jest.fn()
const mockPutUser = jest.fn()

jest.mock('../hooks/useGoogleAuth', () => ({
  useGoogleAuth: () => ({
    user: { id: 'uid-1', email: 'a@b.com', name: 'Ada' },
    authHeaders: {},
    error: null,
    ready: true,
    renderButton: () => {},
    signOut: () => {},
  }),
}))

jest.mock('../lib/api', () => ({
  ...jest.requireActual('../lib/api'),
  createApi: () => ({ getCurrentUser: mockGetCurrentUser, putUser: mockPutUser }),
}))

const profile: User = {
  id: 'uid-1',
  username: 'calm-smiling-kestrel',
  bio: '',
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
    mockGetCurrentUser.mockReset()
    mockPutUser.mockReset()
  })

  it('uses the profile the caller already holds', async () => {
    mockGetCurrentUser.mockResolvedValue(profile)
    renderProvider()

    await waitFor(() =>
      expect(screen.getByTestId('username')).toHaveTextContent('calm-smiling-kestrel'),
    )
    expect(mockPutUser).not.toHaveBeenCalled()
  })

  // Signing in is the only "sign-up" the backend sees, so a caller with no profile gets one here
  // rather than having to visit the editor first - otherwise their posts have no author to show.
  it('creates a profile when the caller has none', async () => {
    mockGetCurrentUser.mockRejectedValue(new ApiError(404, 'user not found'))
    mockPutUser.mockResolvedValue(profile)
    renderProvider()

    await waitFor(() =>
      expect(screen.getByTestId('username')).toHaveTextContent('calm-smiling-kestrel'),
    )
    // An empty body: the username is the backend's to assign.
    expect(mockPutUser).toHaveBeenCalledWith({})
  })

  // A failure that is not "no profile yet" must not be answered by creating one over the top of a
  // profile that may exist and simply could not be read.
  it('does not create a profile when the lookup fails for another reason', async () => {
    mockGetCurrentUser.mockRejectedValue(new ApiError(500, 'internal error'))
    renderProvider()

    await waitFor(() => expect(screen.getByTestId('username')).toHaveTextContent('none'))
    expect(mockPutUser).not.toHaveBeenCalled()
  })
})
