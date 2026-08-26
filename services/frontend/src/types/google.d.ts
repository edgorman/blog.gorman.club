/**
 * Ambient types for the Google Identity Services client library, loaded from accounts.google.com
 * by index.html rather than bundled.
 * @see https://developers.google.com/identity/gsi/web/reference/js-reference
 */
export interface GoogleCredentialResponse {
  credential: string
  select_by?: string
}

interface GoogleIdConfiguration {
  client_id: string
  callback: (response: GoogleCredentialResponse) => void
  auto_select?: boolean
}

interface GoogleButtonConfiguration {
  type?: 'standard' | 'icon'
  theme?: 'outline' | 'filled_blue' | 'filled_black'
  size?: 'large' | 'medium' | 'small'
}

declare global {
  interface Window {
    google?: {
      accounts: {
        id: {
          initialize: (config: GoogleIdConfiguration) => void
          renderButton: (parent: HTMLElement, options: GoogleButtonConfiguration) => void
          /** Shows the One Tap prompt, and is also what makes `auto_select` apply. */
          prompt: () => void
          disableAutoSelect: () => void
        }
      }
    }
  }
}
