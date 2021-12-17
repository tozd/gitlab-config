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

This is a tool implemented in Go. You can use `go install` to install the latest development version (`main` branch):

```sh
go install gitlab.com/tozd/gitlab/config@latest
```

[Releases page](https://gitlab.com/tozd/gitlab/config/-/releases)
contains a list of stable versions. Each includes:

* Statically compiled binaries.
* Docker images.

There is also a [GitHub read-only mirror available](https://github.com/tozd/gitlab-config),
if you need to fork the project there.

## Usage

The tool provides two main commands:

* `get` allows you to retrieve existing configuration of GitLab project and
  store it into an editable YAML file.
* `set` updates the GitLab project's configuration based on the configuration
  in the file.

This enables multiple workflows:

* You can use `gitlab-config get` to backup project's configuration.
* You can first use `gitlab-config get` and then `gitlab-config set` to
  copy configuration from one project to another.
* You can use `gitlab-config set` inside a CI job to configure the project
  every time configuration file stored in the repository is changed.
* You can have one repository with configuration files for many projects.
  A CI/CD job then configures projects when their configuration files change.
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
    - /gitlab-config

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
