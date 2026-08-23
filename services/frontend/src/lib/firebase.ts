import { initializeApp, type FirebaseOptions } from 'firebase/app'
import { getAuth, signInAnonymously } from 'firebase/auth'
import { getFirestore } from 'firebase/firestore'

/**
 * VITE_FIREBASE_CONFIG is base64-encoded JSON (see push-commit.yaml / promote-release.yaml) - not
 * to keep it secret, Firebase web config is meant to ship in the client bundle, but so the JSON's
 * embedded quotes survive being passed through a Docker build-arg unmangled.
 */
function decodeConfig(raw: string | undefined): FirebaseOptions | null {
  if (!raw) return null
  try {
    return JSON.parse(atob(raw)) as FirebaseOptions
  } catch {
    return null
  }
}

export const firebaseConfig = decodeConfig(import.meta.env.VITE_FIREBASE_CONFIG)

export const app = firebaseConfig ? initializeApp(firebaseConfig) : null
export const auth = app ? getAuth(app) : null
export const db = app ? getFirestore(app) : null

/**
 * Resolves with the signed-in uid once anonymous sign-in completes, or null if Firebase isn't
 * configured (e.g. local dev without VITE_FIREBASE_CONFIG) or sign-in fails. Firestore reads need
 * a signed-in caller regardless of visibility - firestore.rules require `request.auth != null`
 * even for public blogs.
 */
export const authReady: Promise<string | null> = auth
  ? signInAnonymously(auth).then(
      (credential) => credential.user.uid,
      () => null,
    )
  : Promise.resolve(null)
