package config

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePushRulesDocumentation(t *testing.T) {
	t.Parallel()

	data, errE := parsePushRulesDocumentation(testProjects)
	require.NoError(t, errE, "% -+#.1v", errE)
	assert.Equal(t, map[string]string{
		"author_email_regex":            "All commit author emails must match this, for example @my-company.com$. Type: string",
		"branch_name_regex":             "All branch names must match this, for example `(feature. Type: string",
		"commit_committer_check":        "Users can only push commits to this repository if the committer email is one of their own verified emails. Type: boolean",
		"commit_message_negative_regex": "No commit message is allowed to match this, for example ssh\\:\\/\\/. Type: string",
		"commit_message_regex":          "All commit messages must match this, for example Fixed \\d+\\..*. Type: string",
		"deny_delete_tag":               "Deny deleting a tag. Type: boolean",
		"file_name_regex":               "All committed filenames must not match this, for example `(jar. Type: string",
		"max_file_size":                 "Maximum file size (MB). Type: integer",
		"member_check":                  "Restrict commits by author (email) to existing GitLab users. Type: boolean",
		"prevent_secrets":               "GitLab rejects any files that are likely to contain secrets. Type: boolean",
		"reject_unsigned_commits":       "Reject commits when they are not GPG signed. Type: boolean",
	}, data)
}
