# Pages projects + custom domains for the frontend, one pair per environment. Project names must match
# cloudflare_project_name in push-commit.yaml / promote-release.yaml. Assumes the gorman.club zone already
# exists (nameservers pointed at Cloudflare manually, same as the root GCP project below).
data "cloudflare_zone" "gorman_club" {
  filter = {
    name = "gorman.club"
  }
}

locals {
  # record_name is the DNS record name relative to the zone; cloudflare_pages_domain only takes the full hostname.
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

# cloudflare_pages_domain validates the hostname but doesn't create the routing DNS record itself.
# Must be proxied (Universal SSL is only presented on proxied records) and use the project's actual
# `subdomain` (not name+".pages.dev", which Cloudflare may suffix if the name is already claimed) -
# otherwise HTTPS fails with ERR_SSL_VERSION_OR_CIPHER_MISMATCH.
resource "cloudflare_dns_record" "frontend" {
  for_each = local.frontend_environments

  zone_id = data.cloudflare_zone.gorman_club.id
  name    = each.value.record_name
  type    = "CNAME"
  content = cloudflare_pages_project.frontend[each.key].subdomain
  proxied = true
  ttl     = 1
}
