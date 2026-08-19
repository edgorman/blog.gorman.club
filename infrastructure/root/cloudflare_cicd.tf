resource "github_actions_variable" "cloudflare_account_id" {
  repository    = var.github_repository_name
  variable_name = "CLOUDFLARE_ACCOUNT_ID"
  value         = var.cloudflare_account_id
}

resource "github_actions_variable" "cloudflare_api_token" {
  repository    = var.github_repository_name
  variable_name = "CLOUDFLARE_API_TOKEN"
  value         = var.cloudflare_api_token
}
