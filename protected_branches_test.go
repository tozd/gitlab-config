package config

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Protected branches file is from: https://gitlab.com/gitlab-org/gitlab/-/raw/master/doc/api/protected_branches.md
//
//go:embed testdata/protected_branches.md
var testProtectedBranches []byte

func TestParseProtectedBranchesDocumentation(t *testing.T) {
	data, err := parseProtectedBranchesDocumentation(testProtectedBranches)
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{
		"allow_force_push":             "When enabled, members who can push to this branch can also force push. Type: boolean",
		"allowed_to_merge":             "Array of merge access levels, with each described by a hash. Type: array",
		"allowed_to_push":              "Array of push access levels, with each described by a hash. Type: array",
		"allowed_to_unprotect":         "Array of unprotect access levels, with each described by a hash. The access level No access is not available for this field. Type: array",
		"code_owner_approval_required": "Prevent pushes to this branch if it matches an item in the CODEOWNERS file. Defaults to false. Type: boolean",
		"name":                         "The name of the branch. Type: string",
	}, data)
}
