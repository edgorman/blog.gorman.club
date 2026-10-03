import { Component, type ReactNode } from 'react'
import { Link, Route, Routes, useLocation } from 'react-router-dom'
import { NavBar } from './components/NavBar'
import { useApp } from './context/AppContext'
import { AppProvider } from './context/AppProvider'
import { GITHUB_REPO, releaseUrl } from './lib/format'
import { EditPost } from './pages/EditPost'
import { EditProfile } from './pages/EditProfile'
import { Landing } from './pages/Landing'
import { NewPost } from './pages/NewPost'
import { Post } from './pages/Post'
import { Privacy } from './pages/Privacy'
import { Terms } from './pages/Terms'
import { UserProfile } from './pages/UserProfile'

function NotFound() {
  return (
    <div className="page">
      <p className="center-note">Page not found.</p>
      <Link to="/">← Back to feed</Link>
    </div>
  )
}

/**
 * Catches a render error in any page, so the reader gets a message and the navbar rather than a
 * blank tab. The error belongs to the path it happened on, so following a link out clears it.
 */
interface BoundaryProps {
  pathname: string
  children: ReactNode
}

class PageErrorBoundary extends Component<BoundaryProps, { failed: boolean; pathname: string }> {
  state = { failed: false, pathname: this.props.pathname }

  static getDerivedStateFromError() {
    return { failed: true }
  }

  static getDerivedStateFromProps(props: BoundaryProps, state: { pathname: string }) {
    return props.pathname === state.pathname ? null : { failed: false, pathname: props.pathname }
  }

  componentDidCatch(error: unknown) {
    console.error(error)
  }

  render() {
    if (!this.state.failed) return this.props.children
    return (
      <div className="page">
        <p role="alert" className="center-note">
          Something went wrong showing this page.
        </p>
        <Link to="/">← Back to feed</Link>
      </div>
    )
  }
}

function Footer() {
  const { version } = useApp()
  return (
    <footer className="site-footer text-muted">
      <Link to="/privacy">Privacy</Link>
      <Link to="/terms">Terms</Link>
      <a href={`https://github.com/${GITHUB_REPO}`}>GitHub</a>
      {version && <a href={releaseUrl(version)}>{version}</a>}
    </footer>
  )
}

function App({
  backendUrl,
  version,
  environment,
}: {
  backendUrl?: string
  version?: string
  environment?: string
}) {
  const { pathname } = useLocation()
  return (
    <AppProvider backendUrl={backendUrl} version={version} environment={environment}>
      <NavBar />
      <PageErrorBoundary pathname={pathname}>
        <Routes>
          <Route path="/" element={<Landing />} />
          {/* A post is addressed by its slug alone: slugs are unique across every author, so the
              author is who wrote a post rather than part of where it lives. "new" is the editor
              rather than a post - React Router ranks the literal above the wildcard beside it, so
              it wins here, and the backend reserves the slug so no post can claim it either. */}
          <Route path="/post/new" element={<NewPost />} />
          <Route path="/post/:slug" element={<Post />} />
          <Route path="/post/:slug/edit" element={<EditPost />} />
          {/* A profile and its editor both sit under the username they belong to, so the editor is
              a segment after a username rather than a name competing with one. */}
          <Route path="/user/:username" element={<UserProfile />} />
          <Route path="/user/:username/edit" element={<EditProfile />} />
          <Route path="/privacy" element={<Privacy />} />
          <Route path="/terms" element={<Terms />} />
          <Route path="*" element={<NotFound />} />
        </Routes>
      </PageErrorBoundary>
      <Footer />
    </AppProvider>
  )
}

export default App
