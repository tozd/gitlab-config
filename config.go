package config

type Configuration struct {
	Project          map[string]interface{}
	Avatar           string                   `json:",omitempty" yaml:",omitempty"`
	SharedWithGroups []map[string]interface{} `json:"shared_with_groups,omitempty" yaml:"shared_with_groups,omitempty"`
}
