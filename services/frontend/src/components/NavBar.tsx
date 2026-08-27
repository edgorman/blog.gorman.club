import { Link } from 'react-router-dom'
import { useApp } from '../context/AppContext'
import { GoogleSignInButton } from './GoogleSignInButton'

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

/** Sticky top bar: brand, new-post link, theme toggle, and account. */
export function NavBar() {
  const { user, authError, authReady, renderSignInButton, signOut, theme, toggleTheme } = useApp()

  return (
    <nav className="nav">
      <Link to="/" className="nav-brand">
        Gorman Club
      </Link>
      <Link to="/new" className="btn btn-ghost gc-navlink">
        New post
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
        <>
          <button type="button" className="btn btn-ghost" onClick={signOut}>
            Sign out
          </button>
          <Link to={`/profile/${user.id}`} className="gc-avatar" aria-label="Your profile">
            {user.name.charAt(0).toUpperCase()}
          </Link>
        </>
      ) : (
        !authError && <GoogleSignInButton ready={authReady} onRender={renderSignInButton} />
      )}
    </nav>
  )
}
