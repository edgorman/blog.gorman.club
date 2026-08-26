/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** Base URL of the backend service (see /services/backend); unset until it is deployed. */
  readonly VITE_BACKEND_URL?: string
  /** Google OAuth 2.0 client ID. Not a secret - it identifies the app. */
  readonly VITE_GOOGLE_CLIENT_ID?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
