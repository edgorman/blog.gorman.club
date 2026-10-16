# 33. Build only what changed, reuse the rest

Status: Accepted (amends [22](0022-build-once-promote-by-sha.md))

## Context

[22](0022-build-once-promote-by-sha.md) built both images and the frontend on every merge, so every release tag had all three in both projects. A merge touching only the frontend still rebuilt and redeployed the backend and worker.

## Decision

- `services/backend:publish`, `worker:publish` and `services/frontend:publish` are `noop` markers like the deploy markers ([20](0020-deploy-markers.md)): their `inputs` are the build's inputs plus the actions and workflow that build them. `push-commit.yaml`'s `changed` job reads them from the same `moon query affected` it runs for Terraform, against the newest `v*` tag, and outputs that tag's commit as `base_sha`.
- An affected image or bundle builds as before. An unaffected image is `base_sha`'s re-tagged with this commit's SHA in both registries (`backend-deploy`'s `reuse_from`), with no staging deploy, since staging already runs it. An unaffected frontend is `base_sha`'s folder fetched from staging's bucket and published again under this commit, then deployed, so the site always shows the current version. Either falls back to building when `base_sha`'s artifact is missing (a folder past retention).
- So every commit still has all three artifacts by SHA in both projects, the publish jobs still succeed on every merge, and the `pre-release` gates ([23](0023-pre-release-gates.md)) and promotion are unchanged.
- `backend-cloudrun-deploy` skips the deploy when the service's latest ready revision already runs the image's digest. On promotion that skips an unchanged backend or worker; a rollback compares digests the same way. The frontend always deploys on promotion, for its version.

## Consequences

A re-tagged image reports the commit it was built from on `/debug`, not the release's. moon only sees files: a change to the `GOOGLE_CLIENT_ID` variable baked into the bundle does not rebuild it.
