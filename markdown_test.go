package config

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Projects file is from: https://gitlab.com/gitlab-org/gitlab/-/raw/master/doc/api/projects.md
//go:embed testdata/projects.md
var testProjects []byte

// Labels file is from: https://gitlab.com/gitlab-org/gitlab/-/raw/master/doc/api/labels.md
//go:embed testdata/labels.md
var testLabels []byte

func TestParseProjectDocumentation(t *testing.T) {
	data, err := parseProjectDocumentation(testProjects)
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{
		"allow_merge_on_skipped_pipeline":                  "Set whether or not merge requests can be merged with skipped jobs. Type: boolean",
		"approvals_before_merge":                           "How many approvers should approve merge request by default. To configure approval rules, see Merge request approvals API. Type: integer",
		"auto_cancel_pending_pipelines":                    "Auto-cancel pending pipelines. This isn't a boolean, but enabled/disabled. Type: string",
		"auto_devops_deploy_strategy":                      "Auto Deploy strategy (continuous, manual, or timed_incremental). Type: string",
		"auto_devops_enabled":                              "Enable Auto DevOps for this project. Type: boolean",
		"autoclose_referenced_issues":                      "Set whether auto-closing referenced issues on default branch. Type: boolean",
		"avatar":                                           "Image file for avatar of the project. Type: mixed",
		"build_coverage_regex":                             "Test coverage parsing. Type: string",
		"build_git_strategy":                               "The Git strategy. Defaults to fetch. Type: string",
		"build_timeout":                                    "The maximum amount of time, in seconds, that a job can run. Type: integer",
		"builds_access_level":                              "One of disabled, private, or enabled. Type: string",
		"ci_config_path":                                   "The path to CI configuration file. Type: string",
		"ci_default_git_depth":                             "Default number of revisions for shallow cloning. Type: integer",
		"ci_forward_deployment_enabled":                    "When a new deployment job starts, skip older deployment jobs that are still pending. Type: boolean",
		"container_expiration_policy":                      "Update the image cleanup policy for this project. Accepts: cadence (string), keep_n (integer), older_than (string), name_regex (string), name_regex_delete (string), name_regex_keep (string), enabled (boolean). Type: hash",
		"container_registry_access_level":                  "Set visibility of container registry, for this project, to one of disabled, private or enabled. Type: string",
		"default_branch":                                   "The default branch name. Type: string",
		"description":                                      "Short project description. Type: string",
		"emails_disabled":                                  "Disable email notifications. Type: boolean",
		"external_authorization_classification_label":      "The classification label for the project. Type: string",
		"forking_access_level":                             "One of disabled, private, or enabled. Type: string",
		"import_url":                                       "URL to import repository from. Type: string",
		"issues_access_level":                              "One of disabled, private, or enabled. Type: string",
		"issues_template":                                  "Default description for Issues. Description is parsed with GitLab Flavored Markdown. See Templates for issues and merge requests. Type: string",
		"keep_latest_artifact":                             "Disable or enable the ability to keep the latest artifact for this project. Type: boolean",
		"lfs_enabled":                                      "Enable LFS. Type: boolean",
		"merge_commit_template":                            "Template used to create merge commit message in merge requests. (Introduced in GitLab 14.5.) Type: string",
		"merge_method":                                     "Set the merge method used. Type: string",
		"merge_requests_access_level":                      "One of disabled, private, or enabled. Type: string",
		"merge_requests_template":                          "Default description for Merge Requests. Description is parsed with GitLab Flavored Markdown. See Templates for issues and merge requests. Type: string",
		"mirror":                                           "Enables pull mirroring in a project. Type: boolean",
		"mirror_overwrites_diverged_branches":              "Pull mirror overwrites diverged branches. Type: boolean",
		"mirror_trigger_builds":                            "Pull mirroring triggers builds. Type: boolean",
		"mirror_user_id":                                   "User responsible for all the activity surrounding a pull mirror event. (administrators only) Type: integer",
		"only_allow_merge_if_all_discussions_are_resolved": "Set whether merge requests can only be merged when all the discussions are resolved. Type: boolean",
		"only_allow_merge_if_pipeline_succeeds":            "Set whether merge requests can only be merged with successful jobs. Type: boolean",
		"only_mirror_protected_branches":                   "Only mirror protected branches. Type: boolean",
		"operations_access_level":                          "One of disabled, private, or enabled. Type: string",
		"packages_enabled":                                 "Enable or disable packages repository feature. Type: boolean",
		"pages_access_level":                               "One of disabled, private, enabled, or public. Type: string",
		"public_jobs":                                      "If true, jobs can be viewed by non-project members. Type: boolean",
		"remove_source_branch_after_merge":                 "Enable Delete source branch option by default for all new merge requests. Type: boolean",
		"repository_access_level":                          "One of disabled, private, or enabled. Type: string",
		"repository_storage":                               "Which storage shard the repository is on. (administrators only) Type: string",
		"request_access_enabled":                           "Allow users to request member access. Type: boolean",
		"resolve_outdated_diff_discussions":                "Automatically resolve merge request diffs discussions on lines changed with a push. Type: boolean",
		"restrict_user_defined_variables":                  "Allow only users with the Maintainer role to pass user-defined variables when triggering a pipeline. For example when the pipeline is triggered in the UI, with the API, or by a trigger token. Type: boolean",
		"service_desk_enabled":                             "Enable or disable Service Desk feature. Type: boolean",
		"shared_runners_enabled":                           "Enable shared runners for this project. Type: boolean",
		"snippets_access_level":                            "One of disabled, private, or enabled. Type: string",
		"squash_commit_template":                           "Template used to create squash commit message in merge requests. (Introduced in GitLab 14.6.) Type: string",
		"squash_option":                                    "One of never, always, default_on, or default_off. Type: string",
		"suggestion_commit_message":                        "The commit message used to apply merge request suggestions. Type: string",
		"topics":                                           "The list of topics for the project. This replaces any existing topics that are already added to the project. (Introduced in GitLab 14.0.) Type: array",
		"wiki_access_level":                                "One of disabled, private, or enabled. Type: string",
	}, data)
}

func TestParseShareDocumentation(t *testing.T) {
	data, err := parseShareDocumentation(testProjects)
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{
		"expires_at":   "Share expiration date in ISO 8601 format: 2016-09-26. Type: string",
		"group_access": "The access level to grant the group. Type: integer",
		"group_id":     "The ID of the group to share with. Type: integer",
	}, data)
}

func TestParseLabelsDocumentation(t *testing.T) {
	data, err := parseLabelsDocumentation(testLabels)
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{
		"color":       "The color of the label given in 6-digit hex notation with leading '#' sign (for example, #FFAABB) or one of the CSS color names. Type: string",
		"description": "The description of the label. Type: string",
		"id":          "The ID or title of a group's label. Type: integer or string",
		"name":        "The name of the label. Type: string",
		"priority":    "The priority of the label. Must be greater or equal than zero or null to remove the priority. Type: integer",
	}, data)
}
