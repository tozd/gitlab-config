package config

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Projects file is from: https://gitlab.com/gitlab-org/gitlab/-/raw/master/doc/api/projects.md
//
//go:embed testdata/projects.md
var testProjects []byte

func TestParseProjectDocumentation(t *testing.T) {
	t.Parallel()

	data, errE := parseProjectDocumentation(testProjects)
	require.NoError(t, errE, "% -+#.1v", errE)
	assert.Equal(t, map[string]string{
		"allow_merge_on_skipped_pipeline":                  "Set whether or not merge requests can be merged with skipped jobs. Type: boolean",
		"allow_pipeline_trigger_approve_deployment":        "Set whether or not a pipeline triggerer is allowed to approve deployments. Type: boolean",
		"analytics_access_level":                           "One of disabled, private or enabled. Type: string",
		"auto_cancel_pending_pipelines":                    "Auto-cancel pending pipelines. This action toggles between an enabled state and a disabled state; it is not a boolean. Type: string",
		"auto_devops_deploy_strategy":                      "Auto Deploy strategy (continuous, manual, or timed_incremental). Type: string",
		"auto_devops_enabled":                              "Enable Auto DevOps for this project. Type: boolean",
		"autoclose_referenced_issues":                      "Set whether auto-closing referenced issues on default branch. Type: boolean",
		"avatar":                                           "Image file for avatar of the project. Type: mixed",
		"build_git_strategy":                               "The Git strategy. Defaults to fetch. Type: string",
		"build_timeout":                                    "The maximum amount of time, in seconds, that a job can run. Type: integer",
		"builds_access_level":                              "One of disabled, private, or enabled. Type: string",
		"ci_allow_fork_pipelines_to_run_in_parent_project": "Enable or disable running pipelines in the parent project for merge requests from forks. (Introduced in GitLab 15.3.) Type: boolean",
		"ci_config_path":                                   "The path to CI configuration file. Type: string",
		"ci_default_git_depth":                             "Default number of revisions for shallow cloning. Type: integer",
		"ci_forward_deployment_enabled":                    "Enable or disable prevent outdated deployment jobs. Type: boolean",
		"ci_forward_deployment_rollback_allowed":           "Enable or disable allow job retries for rollback deployments. Type: boolean",
		"ci_separated_caches":                              "Set whether or not caches should be separated by branch protection status. Type: boolean",
		"container_expiration_policy":                      "Update the image cleanup policy for this project. Accepts: cadence (string), keep_n (integer), older_than (string), name_regex (string), name_regex_delete (string), name_regex_keep (string), enabled (boolean). Type: hash",
		"container_registry_access_level":                  "Set visibility of container registry, for this project, to one of disabled, private or enabled. Type: string",
		"default_branch":                                   "The default branch name. Type: string",
		"description":                                      "Short project description. Type: string",
		"emails_enabled":                                   "Enable email notifications. Type: boolean",
		"enforce_auth_checks_on_uploads":                   "Enforce auth checks on uploads. Type: boolean",
		"environments_access_level":                        "One of disabled, private, or enabled. Type: string",
		"external_authorization_classification_label":      "The classification label for the project. Type: string",
		"feature_flags_access_level":                       "One of disabled, private, or enabled. Type: string",
		"forking_access_level":                             "One of disabled, private, or enabled. Type: string",
		"group_runners_enabled":                            "Enable group runners for this project. Type: boolean",
		"import_url":                                       "URL the repository was imported from. Type: string",
		"infrastructure_access_level":                      "One of disabled, private, or enabled. Type: string",
		"issue_branch_template":                            "Template used to suggest names for branches created from issues. (Introduced in GitLab 15.6.) Type: string",
		"issues_access_level":                              "One of disabled, private, or enabled. Type: string",
		"issues_template":                                  "Default description for Issues. Description is parsed with GitLab Flavored Markdown. See Templates for issues and merge requests. Type: string",
		"keep_latest_artifact":                             "Disable or enable the ability to keep the latest artifact for this project. Type: boolean",
		"lfs_enabled":                                      "Enable LFS. Type: boolean",
		"merge_commit_template":                            "Template used to create merge commit message in merge requests. (Introduced in GitLab 14.5.) Type: string",
		"merge_method":                                     "Set the merge method used. Type: string",
		"merge_pipelines_enabled":                          "Enable or disable merge pipelines. Type: boolean",
		"merge_requests_access_level":                      "One of disabled, private, or enabled. Type: string",
		"merge_requests_template":                          "Default description for merge requests. Description is parsed with GitLab Flavored Markdown. See Templates for issues and merge requests. Type: string",
		"merge_trains_enabled":                             "Enable or disable merge trains. Type: boolean",
		"mirror":                                           "Enables pull mirroring in a project. Type: boolean",
		"mirror_overwrites_diverged_branches":              "Pull mirror overwrites diverged branches. Type: boolean",
		"mirror_trigger_builds":                            "Pull mirroring triggers builds. Type: boolean",
		"mirror_user_id":                                   "User responsible for all the activity surrounding a pull mirror event. (administrators only) Type: integer",
		"monitor_access_level":                             "One of disabled, private, or enabled. Type: string",
		"mr_default_target_self":                           "For forked projects, target merge requests to this project. If false, the target is the upstream project. Type: boolean",
		"only_allow_merge_if_all_discussions_are_resolved": "Set whether merge requests can only be merged when all the discussions are resolved. Type: boolean",
		"only_allow_merge_if_all_status_checks_passed":     "Indicates that merges of merge requests should be blocked unless all status checks have passed. Defaults to false.Introduced in GitLab 15.5 with feature flag only_allow_merge_if_all_status_checks_passed disabled by default. The feature flag was enabled by default in GitLab 15.9. Type: boolean",
		"only_allow_merge_if_pipeline_succeeds":            "Set whether merge requests can only be merged with successful jobs. Type: boolean",
		"only_mirror_protected_branches":                   "Only mirror protected branches. Type: boolean",
		"packages_enabled":                                 "Enable or disable packages repository feature. Type: boolean",
		"pages_access_level":                               "One of disabled, private, enabled, or public. Type: string",
		"printing_merge_request_link_enabled":              "Show link to create/view merge request when pushing from the command line. Type: boolean",
		"public_jobs":                                      "If true, jobs can be viewed by non-project members. Type: boolean",
		"releases_access_level":                            "One of disabled, private, or enabled. Type: string",
		"remove_source_branch_after_merge":                 "Enable Delete source branch option by default for all new merge requests. Type: boolean",
		"repository_access_level":                          "One of disabled, private, or enabled. Type: string",
		"repository_storage":                               "Which storage shard the repository is on. (administrators only) Type: string",
		"request_access_enabled":                           "Allow users to request member access. Type: boolean",
		"requirements_access_level":                        "One of disabled, private, enabled or public. Type: string",
		"resolve_outdated_diff_discussions":                "Automatically resolve merge request diffs discussions on lines changed with a push. Type: boolean",
		"restrict_user_defined_variables":                  "Allow only users with the Maintainer role to pass user-defined variables when triggering a pipeline. For example when the pipeline is triggered in the UI, with the API, or by a trigger token. Type: boolean",
		"security_and_compliance_access_level":             "(GitLab 14.9 and later) Security and compliance access level. One of disabled, private, or enabled. Type: string",
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
