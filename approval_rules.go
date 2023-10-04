package config

import (
	"fmt"
	"net/http"
	"os"
	"sort"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/xanzy/go-gitlab"
	"gitlab.com/tozd/go/errors"
)

// getApprovalRules populates configuration struct with GitLab's project's merge requests
// approval rules available from GitLab approvals API endpoint.
func (c *GetCommand) getApprovalRules(client *gitlab.Client, configuration *Configuration) (bool, errors.E) { //nolint:unparam
	fmt.Fprintf(os.Stderr, "Getting approval rules...\n")

	configuration.ApprovalRules = []map[string]interface{}{}

	descriptions, errE := getApprovalRulesDescriptions(c.DocsRef)
	if errE != nil {
		return false, errE
	}
	// We need "id" later on.
	if _, ok := descriptions["id"]; !ok {
		return false, errors.New(`"id" field is missing in approval rules descriptions`)
	}
	configuration.ApprovalRulesComment = formatDescriptions(descriptions)

	u := fmt.Sprintf("projects/%s/approval_rules", gitlab.PathEscape(c.Project))
	options := &gitlab.GetProjectApprovalRulesListsOptions{
		PerPage: maxGitLabPageSize,
		Page:    1,
	}

	for {
		req, err := client.NewRequest(http.MethodGet, u, options, nil)
		if err != nil {
			errE := errors.WithMessage(err, "failed to get approval rules")
			errors.Details(errE)["page"] = options.Page
			return false, errE
		}

		approvalRules := []map[string]interface{}{}

		response, err := client.Do(req, &approvalRules)
		if err != nil {
			errE := errors.WithMessage(err, "failed to get approval rules")
			errors.Details(errE)["page"] = options.Page
			return false, errE
		}

		if len(approvalRules) == 0 {
			break
		}

		for _, approvalRule := range approvalRules {
			// Making sure ids are an integer.
			castFloatsToInts(approvalRule)

			for _, ii := range []struct {
				From string
				To   string
			}{
				{"users", "user_ids"},
				{"groups", "group_ids"},
				{"protected_branches", "protected_branch_ids"},
			} {
				approvalRule[ii.To], err = convertNestedObjectsToIds(approvalRule[ii.From])
				if err != nil {
					errE := errors.WithMessagef(err, `unable to convert "%s" to "%s" for approval rule`, ii.From, ii.To)
					errors.Details(errE)["approvalRule"] = approvalRule["id"]
					return false, errE
				}
			}

			// Protected branches which exist are listed when applies_to_all_protected_branches
			// is set to true, but they are ignored. So we remove them here as well.
			appliesToAllProtectedBranches, ok := approvalRule["applies_to_all_protected_branches"]
			if ok {
				a, ok := appliesToAllProtectedBranches.(bool) //nolint:govet
				if ok && a {
					approvalRule["protected_branch_ids"] = []interface{}{}
				}
			}

			// Only retain those keys which can be edited through the API
			// (which are those available in descriptions).
			for key := range approvalRule {
				_, ok = descriptions[key]
				if !ok {
					delete(approvalRule, key)
				}
			}

			id, ok := approvalRule["id"]
			if !ok {
				return false, errors.New(`approval rule is missing field "id"`)
			}
			_, ok = id.(int)
			if !ok {
				errE := errors.New(`approval rule's field "id" is not an integer`)
				errors.Details(errE)["type"] = fmt.Sprintf("%T", id)
				errors.Details(errE)["value"] = id
				return false, errE
			}

			configuration.ApprovalRules = append(configuration.ApprovalRules, approvalRule)
		}

		if response.NextPage == 0 {
			break
		}

		options.Page = response.NextPage
	}

	// We sort by protected branch's id so that we have deterministic order.
	sort.Slice(configuration.ApprovalRules, func(i, j int) bool {
		// We checked that id is int above.
		return configuration.ApprovalRules[i]["id"].(int) < configuration.ApprovalRules[j]["id"].(int) //nolint:forcetypeassert
	})

	return false, nil
}

// parseApprovalRulesDocumentation parses GitLab's documentation in Markdown for
// approvals API endpoint and extracts description of fields used to describe
// payload for project's merge requests approval rules.
func parseApprovalRulesDocumentation(input []byte) (map[string]string, errors.E) {
	keyMapper := func(key string) string {
		switch key {
		case "usernames":
			// We want only "used_ids".
			// See: https://gitlab.com/gitlab-org/gitlab/-/issues/419051
			return ""
		case "report_type":
			// "report_type" will be deprecated and is not needed.
			// See: https://gitlab.com/gitlab-org/gitlab/-/issues/419050
			return ""
		default:
			return key
		}
	}
	newDescriptions, err := parseTable(input, "Create project-level rule", keyMapper)
	if err != nil {
		return nil, err
	}
	editDescriptions, err := parseTable(input, "Update project-level rule", keyMapper)
	if err != nil {
		return nil, err
	}
	// We want to preserve approval rule IDs so we copy edit description for it.
	newDescriptions["id"] = editDescriptions["approval_rule_id"]
	return newDescriptions, nil
}

// getApprovalRulesDescriptions obtains description of fields used to describe payload for
// project's merge requests approval rules from GitLab's documentation for approvals API endpoint.
func getApprovalRulesDescriptions(gitRef string) (map[string]string, errors.E) {
	data, err := downloadFile(fmt.Sprintf("https://gitlab.com/gitlab-org/gitlab/-/raw/%s/doc/api/merge_request_approvals.md", gitRef))
	if err != nil {
		return nil, errors.WithMessage(err, "failed to get approval rules descriptions")
	}
	return parseApprovalRulesDocumentation(data)
}

// updateApprovalRules updates GitLab project's merge requests approvals
// using GitLab approvals API endpoint based on the configuration struct.
func (c *SetCommand) updateApprovalRules(client *gitlab.Client, configuration *Configuration) errors.E {
	if configuration.ApprovalRules == nil {
		return nil
	}

	fmt.Fprintf(os.Stderr, "Updating approval rules...\n")

	options := &gitlab.GetProjectApprovalRulesListsOptions{
		PerPage: maxGitLabPageSize,
		Page:    1,
	}

	approvalRules := []*gitlab.ProjectApprovalRule{}

	for {
		as, response, err := client.Projects.GetProjectApprovalRules(c.Project, options)
		if err != nil {
			errE := errors.WithMessage(err, "failed to get approval rules")
			errors.Details(errE)["page"] = options.Page
			return errE
		}

		approvalRules = append(approvalRules, as...)

		if response.NextPage == 0 {
			break
		}

		options.Page = response.NextPage
	}

	existingApprovalRulesSet := mapset.NewThreadUnsafeSet[int]()
	namesToIDs := map[string]int{}
	for _, approvalRule := range approvalRules {
		namesToIDs[approvalRule.Name] = approvalRule.ID
		existingApprovalRulesSet.Add(approvalRule.ID)
	}

	// Set approval rule IDs if a matching existing approval rule can be found.
	for i, approvalRule := range configuration.ApprovalRules { //nolint:dupl
		// Is approval rule ID already set?
		id, ok := approvalRule["id"]
		if ok {
			// If ID is provided, the approval rule should exist.
			iid, ok := id.(int) //nolint:govet
			if !ok {
				errE := errors.New(`approval rule's field "id" is not an integer`)
				errors.Details(errE)["index"] = i
				errors.Details(errE)["type"] = fmt.Sprintf("%T", id)
				errors.Details(errE)["value"] = id
				return errE
			}
			if existingApprovalRulesSet.Contains(iid) {
				continue
			}
			// Approval rule does not exist with that ID. We remove the ID and leave to matching to
			// find the correct ID, if it exists. Otherwise we will just create a new approval rule.
			delete(approvalRule, "id")
		}

		name, ok := approvalRule["name"]
		if !ok {
			errE := errors.Errorf(`approval rule is missing field "name"`)
			errors.Details(errE)["index"] = i
			return errE
		}
		n, ok := name.(string)
		if !ok {
			errE := errors.New(`approval rule's field "name" is not a string`)
			errors.Details(errE)["index"] = i
			errors.Details(errE)["type"] = fmt.Sprintf("%T", name)
			errors.Details(errE)["value"] = name
			return errE
		}
		id, ok = namesToIDs[n]
		if ok {
			approvalRule["id"] = id
		}
	}

	wantedApprovalRulesSet := mapset.NewThreadUnsafeSet[int]()
	for _, approvalRule := range configuration.ApprovalRules {
		id, ok := approvalRule["id"]
		if ok {
			// We checked that id is int above.
			wantedApprovalRulesSet.Add(id.(int)) //nolint:forcetypeassert
		}
	}

	extraApprovalRulesSet := existingApprovalRulesSet.Difference(wantedApprovalRulesSet)
	for _, approvalRuleID := range extraApprovalRulesSet.ToSlice() {
		_, err := client.Projects.DeleteProjectApprovalRule(c.Project, approvalRuleID, nil)
		if err != nil {
			errE := errors.WithMessage(err, "failed to delete approval rule")
			errors.Details(errE)["approvalRule"] = approvalRuleID
			return errE
		}
	}

	for i, approvalRule := range configuration.ApprovalRules {
		// It seems that when rule_type is set to report_approver,
		// an extra report_type field has to be set to code_coverage.
		// "report_type" will eventually be deprecated.
		// See: https://gitlab.com/gitlab-org/gitlab/-/issues/419050
		if approvalRule["rule_type"] == "report_approver" {
			approvalRule["report_type"] = "code_coverage"
		}

		id, ok := approvalRule["id"]
		if !ok { //nolint:dupl
			u := fmt.Sprintf("projects/%s/approval_rules", gitlab.PathEscape(c.Project))
			req, err := client.NewRequest(http.MethodPost, u, approvalRule, nil)
			if err != nil {
				// We made sure above that all approval rules in configuration without approval rule ID have name.
				errE := errors.WithMessage(err, "failed to create approval rule")
				errors.Details(errE)["index"] = i
				errors.Details(errE)["approvalRule"] = approvalRule["name"]
				return errE
			}
			_, err = client.Do(req, nil)
			if err != nil {
				// We made sure above that all approval rules in configuration without approval rule ID have name.
				errE := errors.WithMessage(err, "failed to create approval rule")
				errors.Details(errE)["index"] = i
				errors.Details(errE)["approvalRule"] = approvalRule["name"]
				return errE
			}
		} else {
			// We made sure above that all approval rules in configuration with approval rule
			// ID exist and that they are ints.
			iid := id.(int) //nolint:errcheck,forcetypeassert
			u := fmt.Sprintf("projects/%s/approval_rules/%d", gitlab.PathEscape(c.Project), iid)
			req, err := client.NewRequest(http.MethodPut, u, approvalRule, nil)
			if err != nil {
				errE := errors.WithMessage(err, "failed to update approval rule")
				errors.Details(errE)["index"] = i
				errors.Details(errE)["approvalRule"] = iid
				return errE
			}
			_, err = client.Do(req, nil)
			if err != nil {
				errE := errors.WithMessage(err, "failed to update approval rule")
				errors.Details(errE)["index"] = i
				errors.Details(errE)["approvalRule"] = iid
				return errE
			}
		}
	}

	return nil
}
