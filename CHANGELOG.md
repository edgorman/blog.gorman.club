# Changelog

## [1.0.2](https://github.com/edgorman/blog.gorman.club/compare/v1.0.1...v1.0.2) (2026-11-18)


### Continuous Integration

* share moon setup and affected queries, merge promote services, drop setup-gcloud ([#358](https://github.com/edgorman/blog.gorman.club/issues/358)) ([868720f](https://github.com/edgorman/blog.gorman.club/commit/868720fd5c361a7b6376279c762173b39718eadd))
* skip the secrets scan on docs-only changes, lint Markdown ([#357](https://github.com/edgorman/blog.gorman.club/issues/357)) ([7ded901](https://github.com/edgorman/blog.gorman.club/commit/7ded901bc474455513421ec0f3d9474c94535b52))


### Documentation

* explain CI/CD, normal releases and manual rollbacks ([#355](https://github.com/edgorman/blog.gorman.club/issues/355)) ([50f3114](https://github.com/edgorman/blog.gorman.club/commit/50f311489e3b06d7def047748a576a353a7b938b))

## [1.0.1](https://github.com/edgorman/blog.gorman.club/compare/v1.0.0...v1.0.1) (2026-11-17)


### Bug Fixes

* **ci:** release build, chore, ci, docs and refactor commits ([#342](https://github.com/edgorman/blog.gorman.club/issues/342)) ([1ae0570](https://github.com/edgorman/blog.gorman.club/commit/1ae057096dbacdc0e740bb7c9609f63b3135b1b3))


### Miscellaneous Chores

* **ci:** group Renovate PRs by area ([#337](https://github.com/edgorman/blog.gorman.club/issues/337)) ([9305475](https://github.com/edgorman/blog.gorman.club/commit/93054752e55254fd5e42478a14643e222229df11))
* **deps:** combine open Renovate PRs ([#341](https://github.com/edgorman/blog.gorman.club/issues/341)) ([f2c4ae6](https://github.com/edgorman/blog.gorman.club/commit/f2c4ae666d0f1d89c26cc55cf75098379ae5fc60))
* **infra:** remove the legacy github-actions service account ([#319](https://github.com/edgorman/blog.gorman.club/issues/319)) ([456df38](https://github.com/edgorman/blog.gorman.club/commit/456df38a3f46a2893d5780a491f9115919062b0b))


### Continuous Integration

* deploy the backend, worker and frontend as one services matrix ([#347](https://github.com/edgorman/blog.gorman.club/issues/347)) ([0678cf3](https://github.com/edgorman/blog.gorman.club/commit/0678cf328c33afe9d5a097a4646f2298548500a7))
* gate push-commit jobs with !failure(), use the shared moon-affected action ([#354](https://github.com/edgorman/blog.gorman.club/issues/354)) ([ea81e12](https://github.com/edgorman/blog.gorman.club/commit/ea81e120a3067be843166066b0403c2dadba2ce6))
* let moon decide which jobs run in pull requests and merges ([#346](https://github.com/edgorman/blog.gorman.club/issues/346)) ([b7686c0](https://github.com/edgorman/blog.gorman.club/commit/b7686c00b9f566e3f0cc06f6e481b4165f50b02f))
* read secrets with the standard Secret Manager action ([#350](https://github.com/edgorman/blog.gorman.club/issues/350)) ([7682f7a](https://github.com/edgorman/blog.gorman.club/commit/7682f7a8cabbf4d897138f0dfaf27776af733841))
* remove dead bootstrap and merge_group code, define moon ci targets once ([#349](https://github.com/edgorman/blog.gorman.club/issues/349)) ([1716e06](https://github.com/edgorman/blog.gorman.club/commit/1716e06286bf86a79357d611cca17f65e449d09c))
* share the frontend artifact steps, simplify the apply step, read Node from .prototools ([#353](https://github.com/edgorman/blog.gorman.club/issues/353)) ([ae37b48](https://github.com/edgorman/blog.gorman.club/commit/ae37b481abb0479de4b62408c7f1d2efef4733d8))
* share the moon affected setup, update one plan comment per environment ([#352](https://github.com/edgorman/blog.gorman.club/issues/352)) ([10165a8](https://github.com/edgorman/blog.gorman.club/commit/10165a8598440e5e3ff50d93d165ec1fb095b02f))
* sign in to Google Cloud once per job ([#351](https://github.com/edgorman/blog.gorman.club/issues/351)) ([776c3fc](https://github.com/edgorman/blog.gorman.club/commit/776c3fc36ccd0c1242223461f1450c1ed5bf57c1))
* skip security scanners and infra jobs a change does not touch ([#343](https://github.com/edgorman/blog.gorman.club/issues/343)) ([738c0b1](https://github.com/edgorman/blog.gorman.club/commit/738c0b144d9551803ebe19a7fd62439912fb1ec8))
* split push-commit into release and general workflows ([#348](https://github.com/edgorman/blog.gorman.club/issues/348)) ([4ed0352](https://github.com/edgorman/blog.gorman.club/commit/4ed0352fa3f2df98380b07f8e2444bdbc9449cbe))


### Documentation

* make CI and infrastructure comments declarative ([#320](https://github.com/edgorman/blog.gorman.club/issues/320)) ([1c033db](https://github.com/edgorman/blog.gorman.club/commit/1c033dbb9511495806945ad5d8b34f1040df3ee4))

## [1.0.0](https://github.com/edgorman/blog.gorman.club/compare/v0.31.4...v1.0.0) (2026-11-17)


### ⚠ BREAKING CHANGES

* **infra:** adopt release-please, Renovate, split CI identities and Workers ([#317](https://github.com/edgorman/blog.gorman.club/issues/317))

### Features

* **infra:** adopt release-please, Renovate, split CI identities and Workers ([#317](https://github.com/edgorman/blog.gorman.club/issues/317)) ([9c3a1b0](https://github.com/edgorman/blog.gorman.club/commit/9c3a1b02245384f41cee501894c68bfcfffc5331))
