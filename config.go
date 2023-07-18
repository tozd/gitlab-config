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
	Project                  map[string]interface{}   `json:"project"                               yaml:"project"`
	Avatar                   *string                  `json:"avatar"                                yaml:"avatar"`
	SharedWithGroups         []map[string]interface{} `json:"shared_with_groups"                    yaml:"shared_with_groups"`
	SharedWithGroupsComment  string                   `json:"comment:shared_with_groups,omitempty"  yaml:"comment:shared_with_groups,omitempty"`
	Approvals                map[string]interface{}   `json:"approvals"                             yaml:"approvals"`
	ApprovalRules            []map[string]interface{} `json:"approval_rules"                        yaml:"approval_rules"`
	ApprovalRulesComment     string                   `json:"comment:approval_rules,omitempty"      yaml:"comment:approval_rules,omitempty"`
	ForkedFromProject        *int                     `json:"forked_from_project"                   yaml:"forked_from_project"`
	ForkedFromProjectComment string                   `json:"comment:forked_from_project,omitempty" yaml:"comment:forked_from_project,omitempty"`
	Labels                   []map[string]interface{} `json:"labels"                                yaml:"labels"`
	LabelsComment            string                   `json:"comment:labels,omitempty"              yaml:"comment:labels,omitempty"`
	ProtectedBranches        []map[string]interface{} `json:"protected_branches"                    yaml:"protected_branches"`
	ProtectedBranchesComment string                   `json:"comment:protected_branches,omitempty"  yaml:"comment:protected_branches,omitempty"`
	ProtectedTags            []map[string]interface{} `json:"protected_tags"                        yaml:"protected_tags"`
	ProtectedTagsComment     string                   `json:"comment:protected_tags,omitempty"      yaml:"comment:protected_tags,omitempty"`
	Variables                []map[string]interface{} `json:"variables"                             yaml:"variables"`
	VariablesComment         string                   `json:"comment:variables,omitempty"           yaml:"comment:variables,omitempty"`
}
