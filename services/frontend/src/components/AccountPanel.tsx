import { useEffect } from 'react'
import { Link } from 'react-router-dom'
import { useApp } from '../context/AppContext'
import { GoogleSignInButton } from './GoogleSignInButton'

const CloseIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M18 6 6 18" />
    <path d="m6 6 12 12" />
  </svg>
)

interface Props {
  onClose: () => void
}

/** The account overlay opened from NavBar's account button: sign in/out and a New post shortcut. */
export function AccountPanel({ onClose }: Props) {
  const { user, authError, authReady, renderSignInButton, signOut } = useApp()

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', onKeyDown)
    return () => document.removeEventListener('keydown', onKeyDown)
  }, [onClose])

  return (
    <div className="panel-backdrop" onClick={onClose}>
      <div className="panel-drawer" role="dialog" aria-label="Account" onClick={(e) => e.stopPropagation()}>
        <div className="panel-drawer-header">
          <span className="panel-drawer-title">Account</span>
          <button type="button" className="btn btn-icon btn-secondary" aria-label="Close" onClick={onClose}>
            <CloseIcon />
          </button>
        </div>

        {user ? (
          <>
            <div className="panel-identity">
              <div className="gc-avatar" aria-hidden="true">
                {user.name.charAt(0).toUpperCase()}
              </div>
              <div>
                <div className="panel-identity-name">{user.name}</div>
                <div className="text-muted panel-identity-email">{user.email}</div>
              </div>
            </div>
            <Link to="/new" className="btn btn-primary btn-block" onClick={onClose}>
              New post
            </Link>
            <Link to={`/profile/${user.id}`} className="btn btn-secondary btn-block" onClick={onClose}>
              View profile
            </Link>
            <button type="button" className="btn btn-ghost btn-block" onClick={() => { signOut(); onClose() }}>
              Sign out
            </button>
          </>
        ) : (
          <>
            <p className="text-muted">Sign in to publish and manage your posts.</p>
            {authError ? <p role="alert">{authError}</p> : <GoogleSignInButton ready={authReady} onRender={renderSignInButton} />}
          </>
        )}
      </div>
    </div>
  )
}
