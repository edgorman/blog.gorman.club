import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import './index.css'
import App from './App.tsx'
import { resolveBackendUrl } from './lib/config.ts'

// Resolved before the first render so every page sees a settled backend URL rather than one that
// could change underneath it - see lib/config.ts.
resolveBackendUrl().then((backendUrl) => {
  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <BrowserRouter>
        <App backendUrl={backendUrl} />
      </BrowserRouter>
    </StrictMode>,
  )
})
