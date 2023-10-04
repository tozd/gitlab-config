package config

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Protected tags file is from: https://gitlab.com/gitlab-org/gitlab/-/raw/master/doc/api/pipeline_schedules.md
//
//go:embed testdata/pipeline_schedules.md
var testPipelineSchedules []byte

func TestParsePipelineSchedulesDocumentation(t *testing.T) {
	t.Parallel()

	data, err := parsePipelineSchedulesDocumentation(testPipelineSchedules)
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{
		"active":        "The activation of pipeline schedule. If false is set, the pipeline schedule is initially deactivated. Type: boolean",
		"cron":          "The cron schedule, for example: 0 1 * * *. Type: string",
		"cron_timezone": "The time zone supported by ActiveSupport::TimeZone (for example Pacific Time (US & Canada)), or TZInfo::Timezone (for example America/Los_Angeles). Type: string",
		"description":   "The description of the pipeline schedule. Type: string",
		"id":            "The pipeline schedule ID. Type: integer",
		"ref":           "The branch or tag name that is triggered. Type: string",
		"variables":     "Array of variables, with each described by a hash of the form {key: string, value: string, variable_type: string}. Type: array",
	}, data)
}
