package config

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Protected tags file is from: https://gitlab.com/gitlab-org/gitlab/-/raw/master/doc/api/protected_tags.md
//
//go:embed testdata/protected_tags.md
var testProtectedTags []byte

func TestParseProtectedTagsDocumentation(t *testing.T) {
	data, err := parseProtectedTagsDocumentation(testProtectedTags)
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{
		"allowed_to_create": "Array of access levels allowed to create tags, with each described by a hash of the form {user_id: integer}, {group_id: integer}, or {access_level: integer}. Type: array",
		"name":              "The name of the tag or wildcard. Type: string",
	}, data)
}
