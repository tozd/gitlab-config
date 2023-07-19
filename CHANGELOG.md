# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- Fix integrated SOPS.

### Added

- Support for merge request approvals and approval rules.

### Changed

- Update used GitLab API version to v16.1.2.
- By default gitlab-config does not use anymore the latest API version but a fixed one
  (you can change the version with `--docs` flag).

## [0.3.4] - 2022-09-30

### Fixed

- Fix fetching of CI/CD variables.
- Improve how protected branches are updated.
  [#18](https://gitlab.com/tozd/gitlab/config/-/issues/18)

### Changed

- Debug Docker images run as root to ease installation of additional packages.
  [#23](https://gitlab.com/tozd/gitlab/config/-/issues/23)
  [!1](https://gitlab.com/tozd/gitlab/config/-/merge_requests/1)

## [0.3.3] - 2022-04-16

### Fixed

- Work when CI/CD is disabled.
  [#22](https://gitlab.com/tozd/gitlab/config/-/issues/22)
- Support `analytics_access_level`, `requirements_access_level`, and `security_and_compliance_access_level`.
  [#2](https://gitlab.com/tozd/gitlab/config/-/issues/2)
  [#8](https://gitlab.com/tozd/gitlab/config/-/issues/8)
  [#15](https://gitlab.com/tozd/gitlab/config/-/issues/15)

## [0.3.2] - 2022-01-05

### Fixed

- Passing arguments to `gitlab-config sops` does not require the use
  of `--` anymore.

## [0.3.1] - 2022-01-04

### Fixed

- Remove the use of replace directive in `go.mod`.
  [#20](https://gitlab.com/tozd/gitlab/config/-/issues/20)

## [0.3.0] - 2022-01-03

### Changed

- Change license to Apache 2.0.
- Write progress to stderr.

## [0.2.0] - 2021-12-23

### Added

- `gitlab-config set` automatically attempts to decrypt the configuration using SOPS.
  This can be disabled by passing `--no-decrypt` to `gitlab-config set`.
- `--enc-suffix` flag to `gitlab-config set` command to configure the field suffix to be
  removed before calling APIs. Useful if `--enc-suffix` has been used with `gitlab-config get`.
  Disabled by default.
- `--enc-suffix` flag to `gitlab-config get` command to configure the suffix to field
  names of sensitive values, marking them for encryption with SOPS. Disabled by default.
- `--enc-comment` flag to `gitlab-config get` command to configure the comment which is
  used to annotate sensitive values, marking them for encryption with SOPS.
  By default `sops:enc` comment is used.
- Integrate [SOPS fork](https://github.com/tozd/sops) as `gitlab-config sops` command.
  The fork supports using comments to select values to encrypt and
  computing MAC only over values which end up encrypted.
- Allow configuring the git reference at which to extract API attributes from GitLab's documentation
  using `--docs-ref` CLI flag. By default `master` is used.
- Report progress when getting or updating.
- Support for project level variables. [#17](https://gitlab.com/tozd/gitlab/config/-/issues/17)
- Support for `printing_merge_request_link_enabled`. [#3](https://gitlab.com/tozd/gitlab/config/-/issues/3)
- Support for protected branches. [#16](https://gitlab.com/tozd/gitlab/config/-/issues/16)

### Changed

- Differentiate between empty (any old configuration should be removed) and null/missing configuration
  sections (the section's configuration should be ignored and not managed by gitlab-config).
  `gitlab-config get` now outputs empty sections instead of skipping them, when there is nothing
  configured for the section.

## [0.1.0] - 2021-12-17

### Added

- First public release.

[unreleased]: https://gitlab.com/tozd/gitlab/config/-/compare/v0.3.4...main
[0.3.4]: https://gitlab.com/tozd/gitlab/config/-/compare/v0.3.3...v0.3.4
[0.3.3]: https://gitlab.com/tozd/gitlab/config/-/compare/v0.3.2...v0.3.3
[0.3.2]: https://gitlab.com/tozd/gitlab/config/-/compare/v0.3.1...v0.3.2
[0.3.1]: https://gitlab.com/tozd/gitlab/config/-/compare/v0.3.0...v0.3.1
[0.3.0]: https://gitlab.com/tozd/gitlab/config/-/compare/v0.2.0...v0.3.0
[0.2.0]: https://gitlab.com/tozd/gitlab/config/-/compare/v0.1.0...v0.2.0
[0.1.0]: https://gitlab.com/tozd/gitlab/config/-/tags/v0.1.0

<!-- markdownlint-disable-file MD024 -->
