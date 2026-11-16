# A Worker per environment, named frontend-<env> to match the cloudflare_project_name inputs in
# push-commit.yaml / promote-release.yaml. Terraform creates the Worker and its custom domain;
# wrangler (services/frontend/wrangler.jsonc, frontend-deploy) only uploads versions of it. Each is
# also served at its workers.dev address.
resource "cloudflare_worker" "frontend" {
  for_each = toset(var.gcp_projects)

  account_id = var.cloudflare_account_id
  name       = "frontend-${each.key}"

  subdomain = {
    enabled = true
  }
}

# A custom domain also creates the DNS record that routes to the Worker, so no cloudflare_dns_record
# for the hostnames (the CNAMEs that pointed at the Pages projects are gone with them).
resource "cloudflare_workers_custom_domain" "frontend" {
  for_each = var.frontend_hostnames

  account_id = var.cloudflare_account_id
  zone_name  = var.cloudflare_zone
  hostname   = each.value
  service    = cloudflare_worker.frontend[each.key].name
}

# Proves ownership of gorman.club to Google Search Console (a Domain property), so the blog can be
# submitted for indexing (#221). Search Console created this record itself when the property was
# verified and it is already in state. The zone id is a variable, not a data source, so a PR plan
# never calls Cloudflare (decision 38).
resource "cloudflare_dns_record" "google_site_verification" {
  zone_id = var.cloudflare_zone_id
  name    = var.cloudflare_zone
  type    = "TXT"
  content = "\"google-site-verification=${var.google_site_verification}\""
  ttl     = 3600
}
