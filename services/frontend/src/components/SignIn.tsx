import { GoogleSignInButton } from './GoogleSignInButton'
import type { UseGoogleAuthResult } from '../hooks/useGoogleAuth'

type Props = Pick<UseGoogleAuthResult, 'user' | 'error' | 'ready' | 'renderButton' | 'signOut'>

/**
 * Sign-in panel. Google Identity Services renders the button itself, so this only hosts it and
 * shows what the returned credential says about the caller.
 */
export function SignIn({ user, error, ready, renderButton, signOut }: Props) {
  return (
    <section className="panel" data-state={error ? 'unconfigured' : undefined}>
      <h2>Sign in</h2>

      {user ? (
        <>
          <dl>
            <dt>id</dt>
            <dd>
              <code>{user.id}</code>
            </dd>
            <dt>email</dt>
            <dd>{user.email}</dd>
            <dt>name</dt>
            <dd>{user.name}</dd>
          </dl>
          <div className="actions">
            <button onClick={signOut}>Sign out</button>
          </div>
        </>
      ) : (
        <>
          <p>
            {error
              ? error
              : 'Signed out. The API rejects every request without a valid Google ID token.'}
          </p>
          {!error && <GoogleSignInButton ready={ready} onRender={renderButton} />}
        </>
      )}
    </section>
  )
}
