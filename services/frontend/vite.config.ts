/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/setupTests.ts'],
    // AppProvider and config.ts fall back to this when no backendUrl/config.json is available,
    // which is what their tests rely on outside of the cases that stub it themselves (see
    // hooks/useGoogleAuth.test.ts for VITE_GOOGLE_CLIENT_ID).
    env: { VITE_BACKEND_URL: 'http://api.test' },
  },
})
