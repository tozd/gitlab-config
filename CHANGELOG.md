# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
- `--enc-suffix` flag to  `gitlab-config get` command to configure the suffix to field
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

[Unreleased]: https://gitlab.com/tozd/gitlab/config/-/compare/v0.3.0...main
[0.3.0]: https://gitlab.com/tozd/gitlab/config/-/compare/v0.2.0...v0.3.0
[0.2.0]: https://gitlab.com/tozd/gitlab/config/-/compare/v0.1.0...v0.2.0
[0.1.0]: https://gitlab.com/tozd/gitlab/config/-/tags/v0.1.0

<!-- markdownlint-disable-file MD024 -->
