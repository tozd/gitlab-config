package config

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Protected branches file is from: https://gitlab.com/gitlab-org/gitlab/-/raw/master/doc/api/protected_branches.md
//go:embed testdata/protected_branches.md
var testProtectedBranches []byte

func TestParseProtectedBranchesDocumentation(t *testing.T) {
	data, err := parseProtectedBranchesDocumentation(testProtectedBranches)
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{
		"allow_force_push":             "Allow all users with push access to force push. (default: false) Type: boolean",
		"allowed_to_merge":             "Array of access levels allowed to merge, with each described by a hash. Type: array",
		"allowed_to_push":              "Array of access levels allowed to push, with each described by a hash. Type: array",
		"allowed_to_unprotect":         "Array of access levels allowed to unprotect, with each described by a hash. Type: array",
		"code_owner_approval_required": "Prevent pushes to this branch if it matches an item in the CODEOWNERS file. (defaults: false) Type: boolean",
		"name":                         "The name of the branch or wildcard. Type: string",
	}, data)
}
