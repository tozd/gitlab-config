package config

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Labels file is from: https://gitlab.com/gitlab-org/gitlab/-/raw/master/doc/api/labels.md
//
//go:embed testdata/labels.md
var testLabels []byte

func TestParseLabelsDocumentation(t *testing.T) {
	t.Parallel()

	data, errE := parseLabelsDocumentation(testLabels)
	assert.NoError(t, errE, "% -+#.1v", errE)
	assert.Equal(t, map[string]string{
		"color":       "The color of the label given in 6-digit hex notation with leading '#' sign (for example, #FFAABB) or one of the CSS color names. Type: string",
		"description": "The description of the label. Type: string",
		"id":          "The ID or title of a group's label. Type: integer or string",
		"name":        "The name of the label. Type: string",
		"priority":    "The priority of the label. Must be greater or equal than zero or null to remove the priority. Type: integer",
	}, data)
}
