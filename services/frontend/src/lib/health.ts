export interface HealthStatus {
  status: string
  timestamp: string
  environment: string
  commit: string
}

/**
 * Calls the backend's Debug Endpoint Contract (see CLAUDE.md), which every
 * backend service exposes at `/health` or `/debug` once deployed.
 */
export async function fetchHealth(
  baseUrl: string,
  signal?: AbortSignal,
): Promise<HealthStatus> {
  const response = await fetch(`${baseUrl.replace(/\/$/, '')}/debug`, { signal })

  if (!response.ok) {
    throw new Error(`Backend responded with ${response.status}`)
  }

  return response.json() as Promise<HealthStatus>
}
