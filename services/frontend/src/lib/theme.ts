import { useCallback, useEffect, useState } from 'react'

export type Theme = 'light' | 'dark'

const STORAGE_KEY = 'gc-theme'
const DARK_QUERY = '(prefers-color-scheme: dark)'

function readOverride(): Theme | null {
  try {
    const value = localStorage.getItem(STORAGE_KEY)
    return value === 'light' || value === 'dark' ? value : null
  } catch {
    return null
  }
}

function writeOverride(theme: Theme | null) {
  try {
    if (theme) localStorage.setItem(STORAGE_KEY, theme)
    else localStorage.removeItem(STORAGE_KEY)
  } catch {
    // Storage unavailable: the choice still applies to this page, it just isn't remembered.
  }
}

/**
 * The colour scheme follows the OS unless the visitor pinned one, which public/theme.js applies
 * before paint through <meta name="color-scheme"> and src/index.css maps onto `color-scheme`.
 * A pin is stored only when it differs from the OS, so undoing it returns to following the OS.
 */
export function useTheme(): { theme: Theme; toggleTheme: () => void } {
  const [override, setOverride] = useState(readOverride)
  const [systemDark, setSystemDark] = useState(() => window.matchMedia(DARK_QUERY).matches)
  const system: Theme = systemDark ? 'dark' : 'light'
  const theme = override ?? system

  useEffect(() => {
    const query = window.matchMedia(DARK_QUERY)
    const onSystem = () => setSystemDark(query.matches)
    // Another tab changed the stored choice.
    const onStorage = (event: StorageEvent) => {
      if (event.key === STORAGE_KEY || event.key === null) setOverride(readOverride())
    }
    query.addEventListener('change', onSystem)
    window.addEventListener('storage', onStorage)
    return () => {
      query.removeEventListener('change', onSystem)
      window.removeEventListener('storage', onStorage)
    }
  }, [])

  useEffect(() => {
    const meta = document.querySelector<HTMLMetaElement>('meta[name="color-scheme"]')
    if (meta) meta.content = override ?? 'light dark'
  }, [override])

  const toggleTheme = useCallback(() => {
    const next: Theme = theme === 'dark' ? 'light' : 'dark'
    const nextOverride = next === system ? null : next
    writeOverride(nextOverride)
    setOverride(nextOverride)
  }, [theme, system])

  return { theme, toggleTheme }
}
