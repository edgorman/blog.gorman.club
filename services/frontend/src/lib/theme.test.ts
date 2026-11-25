import { act, renderHook } from '@testing-library/react'
import { useTheme } from './theme'

function setSystemDark(dark: boolean) {
  vi.spyOn(window, 'matchMedia').mockImplementation(
    (query) =>
      ({
        matches: dark,
        media: query,
        addEventListener: () => {},
        removeEventListener: () => {},
      }) as unknown as MediaQueryList,
  )
}

describe('useTheme', () => {
  let meta: HTMLMetaElement
  beforeEach(() => {
    localStorage.clear()
    meta = document.createElement('meta')
    meta.name = 'color-scheme'
    meta.content = 'light dark'
    document.head.append(meta)
  })
  afterEach(() => {
    meta.remove()
    vi.restoreAllMocks()
  })

  it('follows the OS and pins a scheme only when it differs from it', () => {
    setSystemDark(false)
    const { result } = renderHook(() => useTheme())
    expect(result.current.theme).toBe('light')

    act(() => result.current.toggleTheme())
    expect(result.current.theme).toBe('dark')
    expect(localStorage.getItem('gc-theme')).toBe('dark')
    expect(meta.content).toBe('dark')

    // Back to what the OS says: the pin is dropped rather than stored.
    act(() => result.current.toggleTheme())
    expect(result.current.theme).toBe('light')
    expect(localStorage.getItem('gc-theme')).toBeNull()
    expect(meta.content).toBe('light dark')
  })

  it('starts from the stored choice, and from a dark OS with none', () => {
    setSystemDark(true)
    expect(renderHook(() => useTheme()).result.current.theme).toBe('dark')

    localStorage.setItem('gc-theme', 'light')
    expect(renderHook(() => useTheme()).result.current.theme).toBe('light')
  })
})
