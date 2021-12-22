package config

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenameMapField(t *testing.T) {
	tests := []struct {
		input map[string]interface{}
		from  string
		to    string
		want  map[string]interface{}
	}{
		{
			map[string]interface{}{
				"foo": "bar",
			},
			"foo",
			"zoo",
			map[string]interface{}{
				"zoo": "bar",
			},
		},
		{
			map[string]interface{}{
				"kkk": "bar",
				"nested": map[string]interface{}{
					"foo": "bar",
				},
			},
			"foo",
			"zoo",
			map[string]interface{}{
				"kkk": "bar",
				"nested": map[string]interface{}{
					"zoo": "bar",
				},
			},
		},
		{
			map[string]interface{}{
				"foo": "bar",
				"array": []map[string]interface{}{
					{
						"foo": "bar",
					},
					{
						"foo": "bar",
					},
				},
			},
			"foo",
			"zoo",
			map[string]interface{}{
				"zoo": "bar",
				"array": []map[string]interface{}{
					{
						"zoo": "bar",
					},
					{
						"zoo": "bar",
					},
				},
			},
		},
		{
			map[string]interface{}{
				"foo": "bar",
				"array": []interface{}{
					map[string]interface{}{
						"foo": "bar",
					},
					map[string]interface{}{
						"foo": "bar",
					},
				},
			},
			"foo",
			"zoo",
			map[string]interface{}{
				"zoo": "bar",
				"array": []interface{}{
					map[string]interface{}{
						"zoo": "bar",
					},
					map[string]interface{}{
						"zoo": "bar",
					},
				},
			},
		},
	}

	for k, tt := range tests {
		t.Run(fmt.Sprintf("case=%d", k), func(t *testing.T) {
			renameMapField(tt.input, tt.from, tt.to)
			assert.Equal(t, tt.want, tt.input)
		})
	}
}

func TestRemoveFieldSuffix(t *testing.T) {
	tests := []struct {
		input map[string]interface{}
		want  map[string]interface{}
	}{
		{
			map[string]interface{}{
				"foo_suffix": "bar",
			},
			map[string]interface{}{
				"foo": "bar",
			},
		},
		{
			map[string]interface{}{
				"kkk": "bar",
				"nested": map[string]interface{}{
					"foo_suffix": "bar",
				},
			},
			map[string]interface{}{
				"kkk": "bar",
				"nested": map[string]interface{}{
					"foo": "bar",
				},
			},
		},
		{
			map[string]interface{}{
				"foo_suffix": "bar",
				"array": []map[string]interface{}{
					{
						"foo_suffix": "bar",
					},
					{
						"foo_suffix": "bar",
					},
				},
			},
			map[string]interface{}{
				"foo": "bar",
				"array": []map[string]interface{}{
					{
						"foo": "bar",
					},
					{
						"foo": "bar",
					},
				},
			},
		},
		{
			map[string]interface{}{
				"foo_suffix": "bar",
				"array": []interface{}{
					map[string]interface{}{
						"foo_suffix": "bar",
					},
					map[string]interface{}{
						"foo_suffix": "bar",
					},
				},
			},
			map[string]interface{}{
				"foo": "bar",
				"array": []interface{}{
					map[string]interface{}{
						"foo": "bar",
					},
					map[string]interface{}{
						"foo": "bar",
					},
				},
			},
		},
	}

	for k, tt := range tests {
		t.Run(fmt.Sprintf("case=%d", k), func(t *testing.T) {
			removeFieldSuffix(tt.input, "_suffix")
			assert.Equal(t, tt.want, tt.input)
		})
	}
}
