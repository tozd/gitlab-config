package config

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Merge request approvals file is from: https://gitlab.com/gitlab-org/gitlab/-/raw/master/doc/api/merge_request_approvals.md
//
//go:embed testdata/merge_request_approvals.md
var testMergeRequestApprovals []byte

func TestParseApprovalsDocumentation(t *testing.T) {
	data, err := parseApprovalsDocumentation(testMergeRequestApprovals)
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{
		"disable_overriding_approvers_per_merge_request": "Allow or prevent overriding approvers per merge request. Type: boolean",
		"merge_requests_author_approval":                 "Allow or prevent authors from self approving merge requests; true means authors can self approve. Type: boolean",
		"merge_requests_disable_committers_approval":     "Allow or prevent committers from self approving merge requests. Type: boolean",
		"require_password_to_approve":                    "Require approver to enter a password to authenticate before adding the approval. Type: boolean",
		"reset_approvals_on_push":                        "Reset approvals on a new push. Type: boolean",
		"selective_code_owner_removals":                  "Reset approvals from Code Owners if their files changed. Can be enabled only if reset_approvals_on_push is disabled. Type: boolean",
	}, data)
}
