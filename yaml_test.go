package config

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	loremIpsum = "Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do " +
		"eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad " +
		"minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip " +
		"ex ea commodo consequat. Duis aute irure dolor in reprehenderit in " +
		"voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur " +
		"sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt " +
		"mollit anim id est laborum."
)

func TestFormatDescriptions(t *testing.T) {
	t.Parallel()

	formatted := formatDescriptions(map[string]string{
		"foo": "bar",
		"zoo": "something",
		"aaa": loremIpsum,
	})
	expected := "aaa: Lorem ipsum dolor sit amet, consectetur adipiscing elit, " +
		"sed do eiusmod\n" +
		"tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam,\n" +
		"quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo\n" +
		"consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum\n" +
		"dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident,\n" +
		"sunt in culpa qui officia deserunt mollit anim id est laborum.\n" +
		"foo: bar\n" +
		"zoo: something\n"
	assert.Equal(t, expected, formatted)
}

func TestToConfigurationYAML(t *testing.T) {
	t.Parallel()

	tests := []struct {
		config *Configuration
		output string
	}{
		{
			&Configuration{}, "project: {}\n" +
				"avatar: null\n" +
				"shared_with_groups: []\n" +
				"approvals: {}\n" +
				"approval_rules: []\n" +
				"push_rules: {}\n" +
				"forked_from_project: null\n" +
				"labels: []\n" +
				"protected_branches: []\n" +
				"protected_tags: []\n" +
				"variables: []\n" +
				"pipeline_schedules: []\n",
		},
		{
			&Configuration{
				Project: map[string]interface{}{
					// Without a comment-less first object entry ("aaa"),
					// the top-level comment is not shown (but the comment
					// for the first object entry is shown instead).
					"aaa":           "first",
					"comment:foo":   "comment",
					"foo":           "data",
					"comment:":      "top",
					"long":          "something",
					"comment:long":  loremIpsum,
					"comment:extra": "removed",
				},
				SharedWithGroups: []map[string]interface{}{
					{
						"x":         "y",
						"comment:x": "comment",
						"comment:":  "innert top",
					},
				},
				SharedWithGroupsComment: loremIpsum,
				ApprovalRules: []map[string]interface{}{
					{
						"user_ids": []interface{}{
							"comment:array",
							1,
						},
					},
				},
			},
			"project:\n" +
				"  # top\n" +
				"  aaa: first\n" +
				"  # comment\n" +
				"  foo: data\n" +
				"  # Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor\n" +
				"  # incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis\n" +
				"  # nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat.\n" +
				"  # Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu\n" +
				"  # fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in\n" +
				"  # culpa qui officia deserunt mollit anim id est laborum.\n" +
				"  long: something\n" +
				"avatar: null\n" +
				"# Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor\n" +
				"# incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis\n" +
				"# nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat.\n" +
				"# Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu\n" +
				"# fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in\n" +
				"# culpa qui officia deserunt mollit anim id est laborum.\n" +
				"shared_with_groups:\n" +
				"  # innert top\n" +
				"  - # comment\n" +
				"    x: \"y\"\n" +
				"approvals: {}\n" +
				"approval_rules:\n" +
				"  - user_ids:\n" +
				"      # array\n" +
				"      - 1\n" +
				"push_rules: {}\n" +
				"forked_from_project: null\n" +
				"labels: []\n" +
				"protected_branches: []\n" +
				"protected_tags: []\n" +
				"variables: []\n" +
				"pipeline_schedules: []\n",
		},
	}

	for k, tt := range tests {
		t.Run(fmt.Sprintf("case=%d", k), func(t *testing.T) {
			t.Parallel()

			data, errE := toConfigurationYAML(tt.config)
			require.NoError(t, errE, "% -+#.1v", errE)
			assert.Equal(t, tt.output, string(data))
		})
	}
}
