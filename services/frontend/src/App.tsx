import { Link, Route, Routes } from 'react-router-dom'
import { NavBar } from './components/NavBar'
import { AppProvider } from './context/AppProvider'
import { EditPost } from './pages/EditPost'
import { EditProfile } from './pages/EditProfile'
import { Landing } from './pages/Landing'
import { NewPost } from './pages/NewPost'
import { Post } from './pages/Post'
import { UserProfile } from './pages/UserProfile'

function NotFound() {
  return (
    <div className="page">
      <p className="center-note">Page not found.</p>
      <Link to="/">← Back to feed</Link>
    </div>
  )
}

function App() {
  return (
    <AppProvider>
      <NavBar />
      <Routes>
        <Route path="/" element={<Landing />} />
        <Route path="/post/:id" element={<Post />} />
        <Route path="/post/:id/edit" element={<EditPost />} />
        <Route path="/profile/edit" element={<EditProfile />} />
        <Route path="/profile/:username" element={<UserProfile />} />
        <Route path="/new" element={<NewPost />} />
        <Route path="*" element={<NotFound />} />
      </Routes>
    </AppProvider>
  )
}

export default App
