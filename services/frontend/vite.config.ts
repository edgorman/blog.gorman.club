/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: './src/setupTests.ts',
    // AppProvider reads this at module scope to decide whether a backend is configured. Setting it
    // here lets its tests import the module normally, rather than stubbing the env and reloading
    // it - which would give the reloaded copy its own AppContext and ApiError.
    env: { VITE_BACKEND_URL: 'http://api.test' },
  },
})
