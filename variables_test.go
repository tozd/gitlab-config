package config

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Project level variables file is from: https://gitlab.com/gitlab-org/gitlab/-/raw/master/doc/api/project_level_variables.md
//
//go:embed testdata/project_level_variables.md
var testVariables []byte

func TestParseVariablesDocumentation(t *testing.T) {
	t.Parallel()

	data, errE := parseVariablesDocumentation(testVariables)
	require.NoError(t, errE, "% -+#.1v", errE)
	assert.Equal(t, map[string]string{
		"environment_scope": "The environment_scope of the variable. Default: *. Type: string",
		"description":       "The description of the variable. Default: null. Introduced in GitLab 16.2. Type: string",
		"key":               "The key of a variable; must have no more than 255 characters; only A-Z, a-z, 0-9, and _ are allowed. Type: string",
		"masked":            "Whether the variable is masked. Default: false. Type: boolean",
		"protected":         "Whether the variable is protected. Default: false. Type: boolean",
		"raw":               "Whether the variable is treated as a raw string. Default: false. When true, variables in the value are not expanded. Type: boolean",
		"value":             "The value of a variable. Type: string",
		"variable_type":     "The type of a variable. Available types are: env_var (default) and file. Type: string",
	}, data)
}
