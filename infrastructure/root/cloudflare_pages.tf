# Pages projects + custom domains for the frontend service, one pair per
# environment in var.gcp_projects (root has no application services of its
# own, so there's no "root" entry here). Project names match the
# cloudflare_project_name already hardcoded in push-commit.yaml /
# promote-release.yaml (frontend-stag, frontend-prod) — wrangler pushes
# builds into these same projects, it just doesn't manage the project
# resource or custom domain itself.
#
# The zone (gorman.club) is assumed to already exist in the Cloudflare
# account — pointing its nameservers at Cloudflare is a one-time manual step
# outside Terraform's reach, same rationale as the manually-created root GCP
# project in gcp_project.tf. The frontend itself lives on the "blog"
# subdomain of that zone, not at the zone apex.
data "cloudflare_zone" "gorman_club" {
  filter = {
    name = "gorman.club"
  }
}

locals {
  # "blog" is the site's home within the gorman.club zone; staging gets a
  # further subdomain off of that. record_name is the DNS record name
  # relative to the zone that a cloudflare_pages_domain full hostname
  # doesn't itself imply.
  frontend_environments = {
    stag = { hostname = "staging.blog.${data.cloudflare_zone.gorman_club.name}", record_name = "staging.blog" }
    prod = { hostname = "blog.${data.cloudflare_zone.gorman_club.name}", record_name = "blog" }
  }
}

resource "cloudflare_pages_project" "frontend" {
  for_each = local.frontend_environments

  account_id        = var.cloudflare_account_id
  name              = "frontend-${each.key}"
  production_branch = "main"
}

resource "cloudflare_pages_domain" "frontend" {
  for_each = local.frontend_environments

  account_id   = var.cloudflare_account_id
  project_name = cloudflare_pages_project.frontend[each.key].name
  name         = each.value.hostname
}

# cloudflare_pages_domain validates the hostname but doesn't create the
# DNS record that routes traffic to it - that has to be a separate resource.
# Must be proxied (orange cloud): Cloudflare's Universal SSL certificate for
# this zone is only *presented* to visitors on proxied records - a DNS-only
# (grey cloud) record still resolves, but nothing at the target IP holds a
# certificate for this hostname, so HTTPS requests fail with
# ERR_SSL_VERSION_OR_CIPHER_MISMATCH. Proxying a CNAME to *.pages.dev
# doesn't hit error 1014 (Cross-User Banned) - Cloudflare allowlists its
# own product domains (pages.dev, workers.dev, etc.) as CNAME targets.
resource "cloudflare_dns_record" "frontend" {
  for_each = local.frontend_environments

  zone_id = data.cloudflare_zone.gorman_club.id
  name    = each.value.record_name
  type    = "CNAME"
  content = "${cloudflare_pages_project.frontend[each.key].name}.pages.dev"
  proxied = true
  ttl     = 1
}
