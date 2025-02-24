package config

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Protected branches file is from: https://gitlab.com/gitlab-org/gitlab/-/raw/master/doc/api/protected_branches.md
//
//go:embed testdata/protected_branches.md
var testProtectedBranches []byte

func TestParseProtectedBranchesDocumentation(t *testing.T) {
	t.Parallel()

	data, errE := parseProtectedBranchesDocumentation(testProtectedBranches)
	require.NoError(t, errE, "% -+#.1v", errE)
	assert.Equal(t, map[string]string{
		"allow_force_push":             "When enabled, members who can push to this branch can also force push. Type: boolean",
		"allowed_to_merge":             "Array of merge access levels, with each described by a hash of the form {user_id: integer}, {group_id: integer}, or {access_level: integer}. Type: array",
		"allowed_to_push":              "Array of push access levels, with each described by a hash of the form {user_id: integer}, {group_id: integer}, or {access_level: integer}. Type: array",
		"allowed_to_unprotect":         "Array of unprotect access levels, with each described by a hash of the form {user_id: integer}, {group_id: integer}, {access_level: integer}, or {id: integer, _destroy: true} to destroy an existing access level. The access level No access is not available for this field. Type: array",
		"code_owner_approval_required": "Prevent pushes to this branch if it matches an item in the CODEOWNERS file. Type: boolean",
		"name":                         "The name of the branch or wildcard. Type: string",
	}, data)
}
