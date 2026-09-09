/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: './src/setupTests.ts',
    // AppProvider falls back to this when no backendUrl prop is passed, which is what its own
    // tests and config.test.ts's fallback cases rely on - main.tsx is what resolves config.json
    // into that prop at bootstrap, and isn't exercised by these tests.
    env: { VITE_BACKEND_URL: 'http://api.test' },
  },
})
