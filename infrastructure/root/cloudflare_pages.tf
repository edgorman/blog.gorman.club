# Pages projects and custom domains, one pair per environment; names must match the
# cloudflare_project_name inputs in push-commit.yaml / promote-release.yaml.
data "cloudflare_zone" "gorman_club" {
  filter = {
    name = "gorman.club"
  }
}

locals {
  # record_name is relative to the zone; cloudflare_pages_domain takes the full hostname.
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

# cloudflare_pages_domain validates the hostname but doesn't create the routing record. It must be
# proxied and point at the project's actual `subdomain`, or HTTPS fails with a cipher mismatch.
resource "cloudflare_dns_record" "frontend" {
  for_each = local.frontend_environments

  zone_id = data.cloudflare_zone.gorman_club.id
  name    = each.value.record_name
  type    = "CNAME"
  content = cloudflare_pages_project.frontend[each.key].subdomain
  proxied = true
  ttl     = 1
}
