package config

import (
	"fmt"
	"net/http"
	"sort"

	mapset "github.com/deckarep/golang-set"
	"github.com/xanzy/go-gitlab"
	"gitlab.com/tozd/go/errors"
)

// getProtectedBranches populates configuration struct with configuration available
// from GitLab protected branches API endpoint.
func (c *GetCommand) getProtectedBranches(client *gitlab.Client, configuration *Configuration) errors.E {
	fmt.Printf("Getting protected branches...\n")

	configuration.ProtectedBranches = []map[string]interface{}{}

	descriptions, errE := getProtectedBranchesDescriptions(c.DocsRef)
	if errE != nil {
		return errE
	}

	u := fmt.Sprintf("projects/%s/protected_branches", gitlab.PathEscape(c.Project))
	options := &gitlab.ListProtectedBranchesOptions{
		PerPage: maxGitLabPageSize,
		Page:    1,
	}

	for {
		req, err := client.NewRequest(http.MethodGet, u, options, nil)
		if err != nil {
			return errors.Wrapf(err, `failed to get protected branches, page %d`, options.Page)
		}

		protectedBranches := []map[string]interface{}{}

		response, err := client.Do(req, &protectedBranches)
		if err != nil {
			return errors.Wrapf(err, `failed to get protected branches, page %d`, options.Page)
		}

		if len(protectedBranches) == 0 {
			break
		}

		for _, protectedBranch := range protectedBranches {
			// We rename to be consistent between getting and updating.
			protectedBranch["allowed_to_push"] = protectedBranch["push_access_levels"]
			protectedBranch["allowed_to_merge"] = protectedBranch["merge_access_levels"]
			protectedBranch["allowed_to_unprotect"] = protectedBranch["unprotect_access_levels"]

			// Only retain those keys which can be edited through the API
			// (which are those available in descriptions).
			for key := range protectedBranch {
				_, ok := descriptions[key]
				if !ok {
					delete(protectedBranch, key)
				}
			}

			// Make the description be a comment for the sequence item.
			renameMapField(protectedBranch, "access_level_description", "comment:")

			configuration.ProtectedBranches = append(configuration.ProtectedBranches, protectedBranch)
		}

		if response.NextPage == 0 {
			break
		}

		options.Page = response.NextPage
	}

	// We sort by protected branch's name so that we have deterministic order.
	sort.Slice(configuration.ProtectedBranches, func(i, j int) bool {
		return configuration.ProtectedBranches[i]["name"].(string) < configuration.ProtectedBranches[j]["name"].(string)
	})

	configuration.ProtectedBranchesComment = formatDescriptions(descriptions)

	return nil
}

// parseProtectedBranchesDocumentation parses GitLab's documentation in Markdown for
// protected branches API endpoint and extracts description of fields used to describe
// protected branches.
func parseProtectedBranchesDocumentation(input []byte) (map[string]string, errors.E) {
	return parseTable(input, "Protect repository branches", func(key string) string {
		switch key {
		case "push_access_level", "merge_access_level", "unprotect_access_level":
			// We just want to always use "allowed_to_push", "allowed_to_merge",
			// and "allowed_to_unprotect" to be consistent between getting and updating.
			// So we pass through descriptions of "allowed_to_push", "allowed_to_merge",
			// and "allowed_to_unprotect" and then rename "push_access_levels",
			// "merge_access_levels", and "unprotect_access_levels" to them.
			return ""
		default:
			return key
		}
	})
}

// getProtectedBranchesDescriptions obtains description of fields used to describe
// an individual protected branch from GitLab's documentation for protected branches API endpoint.
func getProtectedBranchesDescriptions(gitRef string) (map[string]string, errors.E) {
	data, err := downloadFile(fmt.Sprintf("https://gitlab.com/gitlab-org/gitlab/-/raw/%s/doc/api/protected_branches.md", gitRef))
	if err != nil {
		return nil, errors.Wrap(err, `failed to get protected branches descriptions`)
	}
	return parseProtectedBranchesDocumentation(data)
}

// updateProtectedBranches updates GitLab project's protected branches using GitLab
// protected branches API endpoint based on the configuration struct.
//
// It first unprotects all protected branches which the project does not have anymore
// configured as protected, and then updates or adds protection for configured
// protected branches. When updating an existing protected branch it briefly umprotects
// the branch and reprotects it with new configuration.
func (c *SetCommand) updateProtectedBranches(client *gitlab.Client, configuration *Configuration) errors.E {
	if configuration.ProtectedBranches == nil {
		return nil
	}

	fmt.Printf("Updating protected branches...\n")

	options := &gitlab.ListProtectedBranchesOptions{
		PerPage: maxGitLabPageSize,
		Page:    1,
	}

	protectedBranches := []*gitlab.ProtectedBranch{}

	for {
		pb, response, err := client.ProtectedBranches.ListProtectedBranches(c.Project, options)
		if err != nil {
			return errors.Wrapf(err, `failed to get protected branches, page %d`, options.Page)
		}

		protectedBranches = append(protectedBranches, pb...)

		if response.NextPage == 0 {
			break
		}

		options.Page = response.NextPage
	}

	existingProtectedBranches := mapset.NewThreadUnsafeSet()
	for _, protectedBranch := range protectedBranches {
		existingProtectedBranches.Add(protectedBranch.Name)
	}
	wantedProtectedBranches := mapset.NewThreadUnsafeSet()
	for i, protectedBranch := range configuration.ProtectedBranches {
		name, ok := protectedBranch["name"]
		if !ok {
			return errors.Errorf(`protected branch in configuration at index %d does not have "name"`, i)
		}
		n, ok := name.(string)
		if !ok {
			return errors.Errorf(`invalid "name" in "protected_branches" at index %d`, i)
		}
		wantedProtectedBranches.Add(n)
	}

	extraProtectedBranches := existingProtectedBranches.Difference(wantedProtectedBranches)
	for _, extraProtectedBranch := range extraProtectedBranches.ToSlice() {
		protectedBranchName := extraProtectedBranch.(string) //nolint:errcheck
		_, err := client.ProtectedBranches.UnprotectRepositoryBranches(c.Project, protectedBranchName)
		if err != nil {
			return errors.Wrapf(err, `failed to unprotect branch "%s"`, protectedBranchName)
		}
	}

	u := fmt.Sprintf("projects/%s/protected_branches", gitlab.PathEscape(c.Project))

	for _, protectedBranch := range configuration.ProtectedBranches {
		// We made sure above that all protected branches in configuration have a string name.
		name := protectedBranch["name"].(string) //nolint:errcheck

		// If project already have this protected branch, we have to
		// first unprotect it to be able to update the protected branch.
		if existingProtectedBranches.Contains(name) {
			_, err := client.ProtectedBranches.UnprotectRepositoryBranches(c.Project, name)
			if err != nil {
				return errors.Wrapf(err, `failed to unprotect group "%s" before reprotecting`, name)
			}
		}

		req, err := client.NewRequest(http.MethodPost, u, protectedBranch, nil)
		if err != nil {
			return errors.Wrapf(err, `failed to protect branch "%s"`, name)
		}
		_, err = client.Do(req, nil)
		if err != nil {
			return errors.Wrapf(err, `failed to protect branch "%s"`, name)
		}
	}

	return nil
}
