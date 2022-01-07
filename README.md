# Configure GitLab project with file

[![Go Report Card](https://goreportcard.com/badge/gitlab.com/tozd/gitlab/config)](https://goreportcard.com/report/gitlab.com/tozd/gitlab/config)
[![pipeline status](https://gitlab.com/tozd/gitlab/config/badges/main/pipeline.svg?ignore_skipped=true)](https://gitlab.com/tozd/gitlab/config/-/pipelines)
[![coverage report](https://gitlab.com/tozd/gitlab/config/badges/main/coverage.svg)](https://gitlab.com/tozd/gitlab/config/-/graphs/main/charts)

Keep your GitLab project's configuration in a file. Put it into a git repository and track all changes
to GitLab project's configuration. Keep it in GitLab's project itself to do MRs against the configuration
and discuss changes. Or make configuration changes at the same time as
related changes to the code. Revert to previous versions of configuration.
Duplicate configuration between projects.
In short, GitLab project's configuration as code.

Features:

* gitlab-config is able to retrieve existing GitLab project's configuration and store it into a YAML file.
* Configuration file is automatically annotated with comments describing each setting.
* gitlab-config can update GitLab project's configuration to match the configuration file.
* Designed to be future proof and many API requests are made directly using fields as stored in the
  configuration file. This means that gitlab-config does not have to be updated to support new
  GitLab API changes.
* Can run as a CI job.

Planned:

* Not all GitLab API endpoints are integrated at this point.
  [#16](https://gitlab.com/tozd/gitlab/config/-/issues/16)
* There are
  [known issues](https://gitlab.com/tozd/gitlab/config/-/issues?label_name%5B%5D=blocked+on+gitlab)
  with some GitLab API endpoints on their side.

## Installation

This is a tool implemented in Go. You can use `go install` to install the latest stable (released) version:

```sh
go install gitlab.com/tozd/gitlab/config/cmd/gitlab-config@latest
```

[Releases page](https://gitlab.com/tozd/gitlab/config/-/releases)
contains a list of stable versions. Each includes:

* Statically compiled binaries.
* Docker images.

To install the latest development version (`main` branch):

```sh
go install gitlab.com/tozd/gitlab/release/cmd/gitlab-release@main
```

There is also a [read-only GitHub mirror available](https://github.com/tozd/gitlab-config),
if you need to fork the project there.

## Usage

The tool provides three commands:

* `get` allows you to retrieve existing configuration of GitLab project and
  store it into an editable YAML file.
* `set` updates the GitLab project's configuration based on the configuration
  in the file.
* `sops` integrates [SOPS fork](https://github.com/tozd/sops) as a command.
  The fork supports using comments to select values to encrypt and
  computing MAC only over values which end up encrypted.

This enables multiple workflows:

* You can use `gitlab-config get` to backup project's configuration.
* You can first use `gitlab-config get` and then `gitlab-config set` to
  copy configuration from one project to another.
* You can use `gitlab-config set` inside a CI job to configure the project
  every time configuration file stored in the repository is changed.
* You can have one repository with configuration files for many projects.
  A CI job then configures projects when their configuration files change.
* Somebody changed project's configuration through web UI and you want to see
  what has changed, comparing `gitlab-config get` output with your backup.

Output of `gitlab-config get` can change through time even if you have not
changed configuration yourself because new GitLab versions can introduce
new configuration options. Regularly run `gitlab-config get` and merge
any changes with your configuration file.

To see configuration options available, run

```sh
gitlab-config --help
```

You can provide some configuration options as environment variables.

You have to provide an [access token](https://docs.gitlab.com/ee/api/index.html#personalproject-access-tokens)
using the `-t/--token` command line flag or `GITLAB_API_TOKEN` environment variable.
Use a [personal access token](https://docs.gitlab.com/ee/user/profile/personal_access_tokens.html)
or [project access token](https://docs.gitlab.com/ee/user/project/settings/project_access_tokens.html) with `api` scope
and at least [maintainer role](https://docs.gitlab.com/ee/user/permissions.html) permissions.

Notes:

* Project's name and visibility can be changed only by owners and is because of that
  not exposed by default in configuration as returned by `gitlab-config get`. You can
  manually add them if you want them in there, but then you have to use an access token
  with owner role even if you are not changing them.
* Fork relationship between projects can be changed only by owners. `gitlab-config get`
  returns it because owner role permissions are required only if you want to change the relationship.
* Project's path cannot be changed through the API. [#13](https://gitlab.com/tozd/gitlab/config/-/issues/13)

### GitLab CI configuration

You can add to your GitLab CI configuration a job like:

```yaml
sync_config:
  stage: deploy

  image:
    name: registry.gitlab.com/tozd/gitlab/config/branch/main:latest-debug
    entrypoint: [""]

  script:
    - /gitlab-config set

  rules:
    - if: '$GITLAB_API_TOKEN && $CI_COMMIT_BRANCH == "main"'
      changes:
        - .gitlab-conf.yml
        - .gitlab-avatar.png
```

Notes:

* Job runs only when `GITLAB_API_TOKEN` is present (e.g., only on protected branches)
  and only on the `main` branch (e.g., one with the latest stable version of the configuration file).
  Change to suit your needs.
* Configure `GITLAB_API_TOKEN` as [GitLab CI/CD variable](https://docs.gitlab.com/ee/ci/variables/index.html).
  Protected and masked.
* The example above uses the latest version of the tool from the `main` branch.
  Consider using a Docker image corresponding to the
  [latest released stable version](https://gitlab.com/tozd/gitlab/config/-/releases).
* Use of `-debug` Docker image is currently required.
  See [this issue](https://gitlab.com/tozd/gitlab/config/-/issues/12) for more details.

The configuration above is suitable when you want to manage configuration of a project
using a configuration file inside project's repository itself.
Downside of this approach is that all CI jobs running on the `main` branch get access to an
access token with at least maintainer role permissions. So care must be taken to control
what runs there.

An alternative is to have a separate repository which contains only the configuration
file for a project (or files, for multiple projects) and you provide the access token only there.
Downside of this approach is that you cannot change code and project's configuration at the same
time through one MR.

## Handling sensitive values

It is important to understand that some configuration values are sensitive and should be handled
with care, e.g., CI/CD variables. There are few options available to you:

* Never make configuration obtained using `gitlab-config get` public and handle the whole file
  in a security-conscious way.
* Do not manage configuration sections which include sensitive values using `gitlab-config`.
  This is supported by removing (or setting to `null`) those sections in the configuration file
  (e.g., `variables: null`). `gitlab-config` will detect that and skip them. This is different
  than setting them to empty values (e.g., `variables: []`) which makes `gitlab-config`
  configure GitLab's project accordingly (e.g., remove all CI/CD variables).
* Encrypt the file or just sensitive values in the file using a suitable tool, e.g.,
  [SOPS](https://github.com/mozilla/sops) which supports encrypting with AWS KMS, GCP KMS,
  Azure Key Vault, age, and PGP.
* Use the [fork of SOPS](https://github.com/tozd/sops) integrated with `gitlab-config` which
  supports using comments to select values to encrypt and computing MAC only over values
  which end up encrypted. This is the option we recommend and `gitlab-config` makes it
  easy to use it.

`gitlab-config get` automatically adds `sops:enc` comments to values which are
known to be generally sensitive (you can use `--enc-comment` flag to control this
behavior, together with alternative `--enc-suffix` flag). But those are just defaults.
You can remove comments for values you know are not sensitive and you can add comments
also to other values which are sensitive in your particular configuration. Once you do that,
run the file through `gitlab-config sops --encrypt` to encrypt sensitive values in the file.
An example using [age](https://github.com/FiloSottile/age):

```sh
$ age-keygen -o keys.txt
Public key: age1ey5p0k4072a3nctp38xz0wh6q93s2h5qwnr0fmftuld8yxfkke9sk47feg
$ cat > .sops.yaml
creation_rules:
  - path_regex: ^\.gitlab-conf\.yml$
    age: age1ey5p0k4072a3nctp38xz0wh6q93s2h5qwnr0fmftuld8yxfkke9sk47feg
^D
$ export SOPS_AGE_KEY_FILE=keys.txt
$ gitlab-config sops --encrypt --mac-only-encrypted --in-place --encrypted-comment-regex sops:enc .gitlab-conf.yml
```

If you want to edit the file decrypted temporarily and re-encrypted on save, you can run:

```sh
SOPS_AGE_KEY_FILE=keys.txt gitlab-config sops .gitlab-conf.yml
```

If you want to simply decrypt the file, run:

```sh
SOPS_AGE_KEY_FILE=keys.txt gitlab-config sops --decrypt --in-place .gitlab-conf.yml
```

You can safely store configuration file with encrypted values into the git repository.
If you want to run gitlab-config inside a CI pipeline, then CI job needs access to the
age private key. You can use gitlab-config itself to configure that. Add to your `.gitlab-conf.yml`

```yaml
variables:
  - environment_scope: '*'
    key: SOPS_AGE_KEY_FILE
    masked: false
    protected: true
    # sops:enc
    value: |
      # created: 2021-12-22T22:06:09+01:00
      # public key: age1ey5p0k4072a3nctp38xz0wh6q93s2h5qwnr0fmftuld8yxfkke9sk47feg
      AGE-SECRET-KEY-<the rest of contents of keys.txt>
    variable_type: file
```

Encrypt it using `gitlab-config sops --encrypt` and use `gitlab-config set` to configure
your GitLab project. After this, CI jobs will be able to run `gitlab-config set` as well,
automatically decrypting `.gitlab-conf.yml` file as needed.

Do keep in mind that given above configuration all CI jobs running on protected branches get access
to the age private key. So care must be taken to control what runs there.

## Why does `gitlab-config get` output a new file and not just updates an existing one?

Because there are many (supported) changes you might have done to the file: change comments,
remove/nullify sections, add SOPS comments or field suffixes, encrypt or not values,
there might even be additional API fields you have added for an updated API endpoint.
Automatically meaningfully merging updates into those changes is not possible.
So just generate a new file and compare with the old version yourself, resolving
differences with your preferred tool.

## Projects configured using this tool

To see projects which use this tool and how their configuration looks like,
check out these projects:

* [This project itself](https://gitlab.com/tozd/gitlab/config/-/blob/main/.gitlab-conf.yml)
* [gitlab-release tool](https://gitlab.com/tozd/gitlab/release/-/blob/main/.gitlab-conf.yml)
* [`gitlab.com/tozd/go/errors` Go package](https://gitlab.com/tozd/go/errors/-/blob/main/.gitlab-conf.yml)

_Feel free to make a merge-request adding yours to the list._

## Related projects

* [GitLabForm](https://github.com/gdubicki/gitlabform) – A similar tool written in Python.
  It supports configuring many projects and not just an individual one like gitlab-config.
  On the other hand gitlab-config tries to be just a simple
  translator between a configuration file and API endpoints and do that well,
  making sure you can safely commit your configuration file into a public git
  repository, if you want.
* [GitLab provider for Terraform](https://registry.terraform.io/providers/gitlabhq/gitlab/latest/docs) –
  Supports configuring GitLab projects using Terraform.
  A big hammer if you just want to use it for one project. Moreover, it
  uses a custom language to define configuration.
