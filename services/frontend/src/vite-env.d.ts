/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** Base URL of the backend service (see /services/backend); unset until it is deployed. */
  readonly VITE_BACKEND_URL?: string
  /** Firebase web API key. Not a secret - see src/lib/firebase.ts. */
  readonly VITE_FIREBASE_API_KEY?: string
  /** Firebase auth domain, e.g. blog-gorman-club-stag.firebaseapp.com. */
  readonly VITE_FIREBASE_AUTH_DOMAIN?: string
  /** GCP/Firebase project ID for this environment. */
  readonly VITE_FIREBASE_PROJECT_ID?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
