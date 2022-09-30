---
stage: Verify
group: Pipeline Authoring
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://about.gitlab.com/handbook/product/ux/technical-writing/#assignments
type: reference, api
---

# Project-level CI/CD variables API **(FREE)**

## List project variables

Get list of a project's variables.

```plaintext
GET /projects/:id/variables
```

| Attribute | Type           | Required | Description                                                                                                                                   |
| --------- | -------------- | -------- | --------------------------------------------------------------------------------------------------------------------------------------------- |
| `id`      | integer/string | yes      | The ID of a project or [URL-encoded NAMESPACE/PROJECT_NAME of the project](index.md#namespaced-path-encoding) owned by the authenticated user |

```shell
curl --header "PRIVATE-TOKEN: <your_access_token>" "https://gitlab.example.com/api/v4/projects/1/variables"
```

```json
[
    {
        "variable_type": "env_var",
        "key": "TEST_VARIABLE_1",
        "value": "TEST_1",
        "protected": false,
        "masked": true,
        "environment_scope": "*"
    },
    {
        "variable_type": "env_var",
        "key": "TEST_VARIABLE_2",
        "value": "TEST_2",
        "protected": false,
        "masked": false,
        "environment_scope": "*"
    }
]
```

## Get a single variable

Get the details of a single variable. If there are multiple variables with the same key,
use `filter` to select the correct `environment_scope`.

```plaintext
GET /projects/:id/variables/:key
```

| Attribute | Type           | Required | Description                                                                                                                                   |
| --------- | -------------- | -------- | --------------------------------------------------------------------------------------------------------------------------------------------- |
| `id`      | integer/string | yes      | The ID of a project or [URL-encoded NAMESPACE/PROJECT_NAME of the project](index.md#namespaced-path-encoding) owned by the authenticated user |
| `key`     | string         | yes      | The `key` of a variable                                                                                                                       |
| `filter`  | hash           | no       | Available filters: `[environment_scope]`. See the [`filter` parameter details](#the-filter-parameter).                                        |

```shell
curl --header "PRIVATE-TOKEN: <your_access_token>" "https://gitlab.example.com/api/v4/projects/1/variables/TEST_VARIABLE_1"
```

```json
{
    "key": "TEST_VARIABLE_1",
    "variable_type": "env_var",
    "value": "TEST_1",
    "protected": false,
    "masked": true,
    "environment_scope": "*"
}
```

## Create a variable

Create a new variable. If a variable with the same `key` already exists, the new variable
must have a different `environment_scope`. Otherwise, GitLab returns a message similar to:
`VARIABLE_NAME has already been taken`.

```plaintext
POST /projects/:id/variables
```

| Attribute           | Type           | Required | Description                                                                                                                                   |
| ------------------- | -------------- | -------- | --------------------------------------------------------------------------------------------------------------------------------------------- |
| `id`                | integer/string | yes      | The ID of a project or [URL-encoded NAMESPACE/PROJECT_NAME of the project](index.md#namespaced-path-encoding) owned by the authenticated user |
| `key`               | string         | yes      | The `key` of a variable; must have no more than 255 characters; only `A-Z`, `a-z`, `0-9`, and `_` are allowed                                 |
| `value`             | string         | yes      | The `value` of a variable                                                                                                                     |
| `variable_type`     | string         | no       | The type of a variable. Available types are: `env_var` (default) and `file`                                                                   |
| `protected`         | boolean        | no       | Whether the variable is protected. Default: `false`                                                                                           |
| `masked`            | boolean        | no       | Whether the variable is masked. Default: `false`                                                                                              |
| `environment_scope` | string         | no       | The `environment_scope` of the variable. Default: `*`                                                                                         |

```shell
curl --request POST --header "PRIVATE-TOKEN: <your_access_token>" \
     "https://gitlab.example.com/api/v4/projects/1/variables" --form "key=NEW_VARIABLE" --form "value=new value"
```

```json
{
    "variable_type": "env_var",
    "key": "NEW_VARIABLE",
    "value": "new value",
    "protected": false,
    "masked": false,
    "environment_scope": "*"
}
```

## Update a variable

Update a project's variable. If there are multiple variables with the same key,
use `filter` to select the correct `environment_scope`.

```plaintext
PUT /projects/:id/variables/:key
```

| Attribute           | Type           | Required | Description                                                                                                                                   |
| ------------------- | -------------- | -------- | --------------------------------------------------------------------------------------------------------------------------------------------- |
| `id`                | integer/string | yes      | The ID of a project or [URL-encoded NAMESPACE/PROJECT_NAME of the project](index.md#namespaced-path-encoding) owned by the authenticated user |
| `key`               | string         | yes      | The `key` of a variable                                                                                                                       |
| `value`             | string         | yes      | The `value` of a variable                                                                                                                     |
| `variable_type`     | string         | no       | The type of a variable. Available types are: `env_var` (default) and `file`                                                                   |
| `protected`         | boolean        | no       | Whether the variable is protected                                                                                                             |
| `masked`            | boolean        | no       | Whether the variable is masked                                                                                                                |
| `environment_scope` | string         | no       | The `environment_scope` of the variable                                                                                                       |
| `filter`            | hash           | no       | Available filters: `[environment_scope]`. See the [`filter` parameter details](#the-filter-parameter).                                        |

```shell
curl --request PUT --header "PRIVATE-TOKEN: <your_access_token>" \
     "https://gitlab.example.com/api/v4/projects/1/variables/NEW_VARIABLE" --form "value=updated value"
```

```json
{
    "variable_type": "env_var",
    "key": "NEW_VARIABLE",
    "value": "updated value",
    "protected": true,
    "masked": false,
    "environment_scope": "*"
}
```

## Delete a variable

Delete a project's variable. If there are multiple variables with the same key,
use `filter` to select the correct `environment_scope`.

```plaintext
DELETE /projects/:id/variables/:key
```

| Attribute | Type           | Required | Description                                                                                                                                   |
| --------- | -------------- | -------- | --------------------------------------------------------------------------------------------------------------------------------------------- |
| `id`      | integer/string | yes      | The ID of a project or [URL-encoded NAMESPACE/PROJECT_NAME of the project](index.md#namespaced-path-encoding) owned by the authenticated user |
| `key`     | string         | yes      | The `key` of a variable                                                                                                                       |
| `filter`  | hash           | no       | Available filters: `[environment_scope]`. See the [`filter` parameter details](#the-filter-parameter).                                        |

```shell
curl --request DELETE --header "PRIVATE-TOKEN: <your_access_token>" "https://gitlab.example.com/api/v4/projects/1/variables/VARIABLE_1"
```

## The `filter` parameter

> - [Introduced](https://gitlab.com/gitlab-org/gitlab/-/merge_requests/34490) in GitLab 13.2.
> - [Feature flag removed](https://gitlab.com/gitlab-org/gitlab/-/issues/227052) in GitLab 13.4.

When multiple variables have the same `key`, [GET](#get-a-single-variable), [PUT](#update-a-variable),
or [DELETE](#delete-a-variable) requests might return:

```plaintext
There are multiple variables with provided parameters. Please use 'filter[environment_scope]'.
```

Use `filter[environment_scope]` to select the variable with the matching `environment_scope` attribute.

For example:

- GET:

  ```shell
  curl --globoff --header "PRIVATE-TOKEN: <your_access_token>" \
       "https://gitlab.example.com/api/v4/projects/1/variables/SCOPED_VARIABLE_1?filter[environment_scope]=production"
  ```

- PUT:

  ```shell
  curl --request PUT --globoff --header "PRIVATE-TOKEN: <your_access_token>" \
       "https://gitlab.example.com/api/v4/projects/1/variables/SCOPED_VARIABLE_1?value=scoped-variable-updated-value&environment_scope=production&filter[environment_scope]=production"
  ```

- DELETE:

  ```shell
  curl --request DELETE --globoff --header "PRIVATE-TOKEN: <your_access_token>" \
       "https://gitlab.example.com/api/v4/projects/1/variables/SCOPED_VARIABLE_1?filter[environment_scope]=production"
  ```
