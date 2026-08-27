import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useApp } from '../context/AppContext'
import { AccountPanel } from './AccountPanel'

const SunIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <circle cx="12" cy="12" r="4" />
    <path d="M12 2v2" />
    <path d="M12 20v2" />
    <path d="m4.93 4.93 1.41 1.41" />
    <path d="m17.66 17.66 1.41 1.41" />
    <path d="M2 12h2" />
    <path d="M20 12h2" />
    <path d="m6.34 17.66-1.41 1.41" />
    <path d="m19.07 4.93-1.41 1.41" />
  </svg>
)

const MoonIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9Z" />
  </svg>
)

const UserIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M20 21a8 8 0 0 0-16 0" />
    <circle cx="12" cy="7" r="4" />
  </svg>
)

/** Sticky top bar: brand, theme toggle, and an account button that opens AccountPanel. */
export function NavBar() {
  const { user, theme, toggleTheme } = useApp()
  const [accountOpen, setAccountOpen] = useState(false)

  return (
    <nav className="nav">
      <Link to="/" className="nav-brand" aria-label="blog, gorman club">
        <span className="nav-brand-main" aria-hidden="true">blog</span>
        <span className="nav-brand-sub" aria-hidden="true">gorman club</span>
      </Link>
      <button
        type="button"
        className="btn btn-icon btn-secondary"
        aria-label="Toggle dark mode"
        onClick={toggleTheme}
      >
        {theme === 'dark' ? <SunIcon /> : <MoonIcon />}
      </button>

      {user ? (
        <button
          type="button"
          className="gc-avatar"
          aria-label="Account"
          aria-haspopup="dialog"
          aria-expanded={accountOpen}
          onClick={() => setAccountOpen(true)}
        >
          {user.name.charAt(0).toUpperCase()}
        </button>
      ) : (
        <button
          type="button"
          className="btn btn-icon btn-secondary"
          aria-label="Account"
          aria-haspopup="dialog"
          aria-expanded={accountOpen}
          onClick={() => setAccountOpen(true)}
        >
          <UserIcon />
        </button>
      )}

      {accountOpen && <AccountPanel onClose={() => setAccountOpen(false)} />}
    </nav>
  )
}
