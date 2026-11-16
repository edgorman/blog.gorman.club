# 36. The frontend runs as a Cloudflare Worker with static assets

Status: Accepted (amends [22](0022-build-once-promote-by-sha.md))

## Context

Cloudflare Pages is in maintenance, and Workers static assets does what the blog uses it for: serving `dist/`, `_headers`, the SPA fallback, and a script in front of them for crawlers (#221). Pages kept the script in `public/_worker.js`, so it travelled inside the bundle.

## Decision

- Each environment is a Worker, `frontend-stag` and `frontend-prod`, created by `infrastructure/root/cloudflare_workers.tf` together with its custom domain (`staging.blog.gorman.club`, `blog.gorman.club`) and a `workers.dev` address. Terraform creates the Worker; wrangler only uploads versions of it.
- The crawler script moved to `services/frontend/worker/index.js`, run first for every request (`wrangler.jsonc`, `run_worker_first`) and handing everything else to the static assets through the `ASSETS` binding. It is deployed from the same commit as the bundle it fronts, so promotion still deploys one commit's SHA ([22](0022-build-once-promote-by-sha.md)).
- `frontend-deploy` runs `wrangler deploy --name <worker> --assets dist`, so one bundle serves every environment. The Cloudflare account id is a plain Actions variable (`CLOUDFLARE_ACCOUNT_ID`) rather than a secret, because plans need it ([38](0038-separate-plan-and-apply-identities.md)).
- The Pages projects, their domains and their CNAME records are deleted by the same root apply. That is the DNS cutover: between that apply and the first `frontend-deploy` the hostnames serve nothing.

## Consequences

The two Terraform-managed Pages DNS records and the `data.cloudflare_zone` lookups are gone; the zone and account ids are in `infrastructure/config/root/terraform.tfvars`. A custom domain on a Worker with no uploaded version may be refused by Cloudflare, so the first apply can need a second run after the first deploy.
