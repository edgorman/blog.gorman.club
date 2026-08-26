/**
 * Ambient types for the Google Identity Services client library, loaded from
 * accounts.google.com by index.html rather than bundled.
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
  cancel_on_tap_outside?: boolean
}

interface GoogleButtonConfiguration {
  type?: 'standard' | 'icon'
  theme?: 'outline' | 'filled_blue' | 'filled_black'
  size?: 'large' | 'medium' | 'small'
  text?: 'signin_with' | 'signup_with' | 'continue_with' | 'signin'
  shape?: 'rectangular' | 'pill' | 'circle' | 'square'
  logo_alignment?: 'left' | 'center'
  width?: string | number
}

declare global {
  interface Window {
    google?: {
      accounts: {
        id: {
          initialize: (config: GoogleIdConfiguration) => void
          renderButton: (parent: HTMLElement, options: GoogleButtonConfiguration) => void
          /**
           * Shows the One Tap prompt. This is also what makes `auto_select` apply: it is a One
           * Tap setting, so a page that only calls renderButton() never consults it.
           */
          prompt: () => void
          disableAutoSelect: () => void
        }
      }
    }
  }
}
