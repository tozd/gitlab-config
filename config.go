package config

type Configuration struct {
	Project                 map[string]interface{}
	Avatar                  string                   `json:",omitempty" yaml:",omitempty"`
	SharedWithGroups        []map[string]interface{} `json:"shared_with_groups,omitempty" yaml:"shared_with_groups,omitempty"`
	SharedWithGroupsComment string                   `json:"comment:shared_with_groups,omitempty" yaml:"comment:shared_with_groups,omitempty"`
	ForkedFromProjectID     int                      `json:"forked_from_project,omitempty" yaml:"forked_from_project,omitempty"`
	ForkedFromProjectPath   string                   `json:"comment:forked_from_project,omitempty" yaml:"comment:forked_from_project,omitempty"`
	Labels                  []map[string]interface{} `json:"labels,omitempty" yaml:"labels,omitempty"`
	LabelsComment           string                   `json:"comment:labels,omitempty" yaml:"comment:labels,omitempty"`
}
