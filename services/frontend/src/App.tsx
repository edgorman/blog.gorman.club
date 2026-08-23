import { HealthCheck } from './components/HealthCheck'
import { Blogs } from './components/Blogs'
import './App.css'

function App() {
  return (
    <main>
      <h1>blog.gorman.club</h1>
      <p>The blog subdomain for the gorman.club website.</p>
      <HealthCheck />
      <Blogs />
    </main>
  )
}

export default App
