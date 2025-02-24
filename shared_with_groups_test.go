package config

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSharedWithGroupsDocumentation(t *testing.T) {
	t.Parallel()

	data, errE := parseSharedWithGroupsDocumentation(testProjects)
	require.NoError(t, errE, "% -+#.1v", errE)
	assert.Equal(t, map[string]string{
		"expires_at":   "Share expiration date in ISO 8601 format: 2016-09-26. Type: string",
		"group_access": "The role (access_level) to grant the group. Type: integer",
		"group_id":     "The ID of the group to share with. Type: integer",
	}, data)
}
