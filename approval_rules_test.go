package config

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseApprovalRulesDocumentation(t *testing.T) {
	t.Parallel()

	data, errE := parseApprovalRulesDocumentation(testMergeRequestApprovals)
	require.NoError(t, errE, "% -+#.1v", errE)
	assert.Equal(t, map[string]string{
		"approvals_required":                "The number of required approvals for this rule. Type: integer",
		"name":                              "The name of the approval rule. Type: string",
		"applies_to_all_protected_branches": "Whether the rule is applied to all protected branches. If set to true, the value of protected_branch_ids is ignored. Default is false. Introduced in GitLab 15.3. Type: boolean",
		"group_ids":                         "The IDs of groups as approvers. Type: Array",
		"protected_branch_ids":              "The IDs of protected branches to scope the rule by. To identify the ID, use the API. Type: Array",
		"rule_type":                         "The type of rule. any_approver is a pre-configured default rule with approvals_required at 0. Other rules are regular and report_approver. Type: string",
		"user_ids":                          "The IDs of users as approvers. If you provide both user_ids and usernames, both lists of users are added. Type: Array",
		"id":                                "The ID of a approval rule. Type: integer",
	}, data)
}
