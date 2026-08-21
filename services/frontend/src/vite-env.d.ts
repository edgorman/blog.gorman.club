/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** Base URL of the backend service (see /services/backend); unset until it is deployed. */
  readonly VITE_BACKEND_URL?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
