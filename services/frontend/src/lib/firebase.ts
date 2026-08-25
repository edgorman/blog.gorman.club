import { initializeApp, type FirebaseApp } from 'firebase/app'
import { getAuth, type Auth } from 'firebase/auth'

/**
 * Firebase is used for authentication only - never for data. Every read and write goes through
 * the backend API (see /services/backend), which verifies the ID token minted here. Nothing in
 * this app imports `firebase/firestore`.
 *
 * Config is baked in at build time, same as VITE_BACKEND_URL. The API key is not a secret for
 * web apps; access is governed by the backend, not by hiding it.
 */
const config = {
  apiKey: import.meta.env.VITE_FIREBASE_API_KEY,
  authDomain: import.meta.env.VITE_FIREBASE_AUTH_DOMAIN,
  projectId: import.meta.env.VITE_FIREBASE_PROJECT_ID,
}

/** False until the build is given a Firebase config, so the UI can say so instead of crashing. */
export const firebaseConfigured = Boolean(
  config.apiKey && config.authDomain && config.projectId,
)

let app: FirebaseApp | undefined
let auth: Auth | undefined

/** Returns the Auth instance, initialising lazily. Throws if the build has no config. */
export function getFirebaseAuth(): Auth {
  if (!firebaseConfigured) {
    throw new Error('Firebase is not configured for this build')
  }
  if (!auth) {
    app = initializeApp(config)
    auth = getAuth(app)
  }
  return auth
}
