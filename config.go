package config

// Configuration represents GitLab's project configuration supported.
//
// Some fields have type map[string]interface{} because they are passed almost as-is
// to GitLab API. This allows for potential customization in behavior beyond what
// is currently supported by this package.
//
// All fields with prefix "comment:" are moved into YAML comments before they are
// written out. Similarly, fields which have "Comment" suffix are moved into
// YAML comments and are not used for project configuration.
type Configuration struct {
	Project                  map[string]interface{}   `json:",omitempty" yaml:",omitempty"`
	Avatar                   string                   `json:",omitempty" yaml:",omitempty"`
	SharedWithGroups         []map[string]interface{} `json:"shared_with_groups,omitempty" yaml:"shared_with_groups,omitempty"`
	SharedWithGroupsComment  string                   `json:"comment:shared_with_groups,omitempty" yaml:"comment:shared_with_groups,omitempty"`
	ForkedFromProject        int                      `json:"forked_from_project,omitempty" yaml:"forked_from_project,omitempty"`
	ForkedFromProjectComment string                   `json:"comment:forked_from_project,omitempty" yaml:"comment:forked_from_project,omitempty"`
	Labels                   []map[string]interface{} `json:"labels,omitempty" yaml:"labels,omitempty"`
	LabelsComment            string                   `json:"comment:labels,omitempty" yaml:"comment:labels,omitempty"`
	ProtectedBranches        []map[string]interface{} `json:"protected_branches,omitempty" yaml:"protected_branches,omitempty"`
	ProtectedBranchesComment string                   `json:"comment:protected_branches,omitempty" yaml:"comment:protected_branches,omitempty"`
	Variables                []map[string]interface{} `json:"variables,omitempty" yaml:"variables,omitempty"`
	VariablesComment         string                   `json:"comment:variables,omitempty" yaml:"comment:variables,omitempty"`
}
