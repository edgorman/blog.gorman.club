/**
 * Runtime configuration written by the deploy pipeline (see the Staging Deployments section of
 * CLAUDE.md) into `config.json`, served alongside the static build.
 */
export interface RuntimeConfig {
  backendUrl?: string
}

/**
 * Resolves the backend URL for this deployment: `config.json`'s `backendUrl` when the file is
 * present and holds a non-empty string, otherwise the build-time `VITE_BACKEND_URL` - so local
 * dev (which has no `config.json`) and a rollback to an image that predates this file both keep
 * working. Called once at bootstrap (see main.tsx), before the app's first render, so every page
 * sees a settled value rather than one that could change underneath it.
 */
export async function resolveBackendUrl(fetchImpl: typeof fetch = fetch): Promise<string | undefined> {
  try {
    const response = await fetchImpl('/config.json')
    if (response.ok) {
      const config = (await response.json()) as RuntimeConfig
      if (typeof config.backendUrl === 'string' && config.backendUrl) return config.backendUrl
    }
  } catch {
    // Missing, unreachable, or not valid JSON - fall through to the build-time value below.
  }
  return import.meta.env.VITE_BACKEND_URL
}
