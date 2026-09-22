/**
 * Runtime configuration written by the deploy pipeline (see the Staging Deployments section of
 * CLAUDE.md) into `config.json`, served alongside the static build. `version` and `environment`
 * are the deploy's own identifiers - a release tag or commit SHA, and 'staging'/'production' -
 * present on every deployed build and absent only in local dev, which has no `config.json`.
 */
export interface RuntimeConfig {
  backendUrl?: string
  version?: string
  environment?: string
}

/**
 * Resolves this deployment's runtime configuration from `config.json`, in one fetch: `backendUrl`
 * falls back to the build-time `VITE_BACKEND_URL` when the file is missing, unreachable, invalid
 * JSON, or holds no non-empty `backendUrl` - so local dev and a rollback to an image that predates
 * this file both keep working - while `version`/`environment` have no build-time fallback and stay
 * undefined in those same cases. Called once at bootstrap (see main.tsx), before the app's first
 * render, so every page sees settled values rather than ones that could change underneath it.
 */
export async function resolveConfig(fetchImpl: typeof fetch = fetch): Promise<RuntimeConfig> {
  let config: RuntimeConfig = {}
  try {
    const response = await fetchImpl('/config.json')
    if (response.ok) config = (await response.json()) as RuntimeConfig
  } catch {
    // Missing, unreachable, or not valid JSON - config stays {} and every field falls back below.
  }
  return {
    backendUrl: typeof config.backendUrl === 'string' && config.backendUrl
      ? config.backendUrl
      : import.meta.env.VITE_BACKEND_URL,
    version: typeof config.version === 'string' && config.version ? config.version : undefined,
    environment: typeof config.environment === 'string' && config.environment ? config.environment : undefined,
  }
}
