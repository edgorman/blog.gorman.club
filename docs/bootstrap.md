# Bootstrap and teardown

How every environment is built from nothing, and how to take it all down again. This is the only Terraform anyone runs by hand: `make init` once to create root, and a teardown when rebuilding. Everything else goes through CI ([`.github/AGENTS.md`](../.github/AGENTS.md)).

The three GCP projects stay across a rebuild. `blog-gorman-club-root` is made by hand and holds the billing link and the OAuth client, and a deleted project's id can never be reused, so deleting `-stag` or `-prod` would mean renaming every environment.

## What you need

- gcloud and the toolchain from [`start-here.md`](start-here.md), logged in with `gcloud auth application-default login` as an owner of the three projects.
- A GitHub personal access token for this repository with read and write access to **Variables**.
- A Cloudflare API token with **Account: Cloudflare Pages: Edit**, **Zone: DNS: Edit** and **Zone: Zone: Read** on `gorman.club`, and the Cloudflare account id.
- The OAuth client ID in `infrastructure/config/root/terraform.tfvars` (`google_client_id`). It lives in the root project and survives a teardown; to make a new one, see "Google Sign-In" in [`services/frontend/README.md`](../services/frontend/README.md).

Don't merge anything to `main` between starting a teardown and finishing step 4 of the bootstrap.

## Bootstrap

1. **Root.** Creates the environment projects (imports root's), every Terraform state bucket, each environment's artifact registry and frontend bucket ([34](decisions/0034-artifact-stores-in-root.md)), Workload Identity Federation, the Cloudflare Pages projects and DNS records, the three credentials in Secret Manager and the GitHub Actions variables. Then it moves its own state into the bucket it made.

   ```sh
   cd infrastructure/root
   TF_VAR_github_provider_token=<PAT> TF_VAR_cloudflare_account_id=<ID> TF_VAR_cloudflare_api_token=<TOKEN> make init
   ```

2. **Staging.** Merge a pull request that changes `infrastructure/env`, or anything else that marks `deploy-staging` affected. The merge applies staging from nothing, builds and deploys both images and the frontend, runs the smoke test and cuts a pre-release. Images missing from the last release (after a teardown, all of them) are built rather than reused.
3. **Prod.** Promote that pre-release to a release. Promotion applies prod's Terraform, then deploys by commit SHA.
4. **By hand.** Sign in on both sites, and set your own `subscribedUntil` in Firestore to turn the writing assistant back on (it is hand-set, see `services/backend/README.md`).

Releases from before a teardown can't be promoted afterwards: their images went with the registries.

## Teardown

Done from a local checkout of `main`. The edits below lift deletion protection, so they are never committed: a merge would apply them to the live environments.

### Prod, then staging

For each `<env>` in `prod`, then `staging`:

1. In `infrastructure/env`, edit locally:
   - `firestore.tf`: delete the `lifecycle { prevent_destroy = true }` block, and set `delete_protection_state = "DELETE_PROTECTION_DISABLED"` and `deletion_policy = "DELETE"`.
   - `cloud_run.tf` and `worker.tf`: add `deletion_protection = false` to both `google_cloud_run_v2_service` resources.
2. Apply those edits, which turns the protection off, then destroy:

   ```sh
   cd infrastructure/env
   rm -rf .terraform
   terraform init -backend-config=../config/<env>/terraform.tfbackend
   export TF_VAR_google_client_id=<google_client_id from config/root/terraform.tfvars>
   terraform apply -var-file=../config/<env>/terraform.tfvars
   terraform destroy -var-file=../config/<env>/terraform.tfvars
   ```

### Root

Root's state lives in a bucket it destroys, so first move the state to local (the reverse of `make init`):

1. In `infrastructure/root`, edit locally:
   - `providers.tf`: delete the `backend "gcs"` block.
   - `gcp_project.tf`: add `deletion_policy = "ABANDON"` to both `google_project` resources, so the projects are dropped from state rather than deleted, and `disable_on_destroy = false` to `google_project_service.project_services`, so the kept projects' baseline APIs stay on while the resources using them are deleted.
   - `gcp_project.tf` and `artifact_stores.tf`: set `force_destroy = true` on all three buckets.
2. Migrate, apply the edits, destroy:

   ```sh
   cd infrastructure/root
   rm -rf .terraform
   terraform init -migrate-state -force-copy
   export TF_VAR_github_provider_token=<PAT> TF_VAR_cloudflare_account_id=<ID> TF_VAR_cloudflare_api_token=<TOKEN>
   terraform apply -var-file=../config/root/terraform.tfvars
   terraform destroy -var-file=../config/root/terraform.tfvars
   ```

3. Clean up: `git checkout -- .` and `rm -rf .terraform terraform.tfstate*` in `infrastructure/root` and `infrastructure/env`.

Both sites are down from here until the bootstrap's step 2.

A destroyed Workload Identity pool keeps its id for 30 days. To bootstrap again sooner, give `google_iam_workload_identity_pool.github_pool` a new `workload_identity_pool_id` in `infrastructure/root/github_cicd.tf` first.
