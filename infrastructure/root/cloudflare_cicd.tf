resource "github_actions_variable" "cloudflare_account_id" {
  repository    = var.github_repository_name
  variable_name = "CLOUDFLARE_ACCOUNT_ID"
  value         = var.cloudflare_account_id
}

# A credential, not a plain id, so this is an encrypted Actions secret
# (github_actions_secret) rather than a plaintext Actions variable.
resource "github_actions_secret" "cloudflare_api_token" {
  repository      = var.github_repository_name
  secret_name     = "CLOUDFLARE_API_TOKEN"
  plaintext_value = var.cloudflare_api_token
}
