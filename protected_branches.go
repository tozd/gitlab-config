package config

import (
	"fmt"
	"net/http"
	"os"
	"sort"

	mapset "github.com/deckarep/golang-set"
	"github.com/xanzy/go-gitlab"
	"gitlab.com/tozd/go/errors"
)

// getProtectedBranches populates configuration struct with configuration available
// from GitLab protected branches API endpoint.
func (c *GetCommand) getProtectedBranches(client *gitlab.Client, configuration *Configuration) (bool, errors.E) {
	fmt.Fprintf(os.Stderr, "Getting protected branches...\n")

	configuration.ProtectedBranches = []map[string]interface{}{}

	descriptions, errE := getProtectedBranchesDescriptions(c.DocsRef)
	if errE != nil {
		return false, errE
	}
	configuration.ProtectedBranchesComment = formatDescriptions(descriptions)

	u := fmt.Sprintf("projects/%s/protected_branches", gitlab.PathEscape(c.Project))
	options := &gitlab.ListProtectedBranchesOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: maxGitLabPageSize,
			Page:    1,
		},
	}

	for {
		req, err := client.NewRequest(http.MethodGet, u, options, nil)
		if err != nil {
			return false, errors.Wrapf(err, `failed to get protected branches, page %d`, options.Page)
		}

		protectedBranches := []map[string]interface{}{}

		response, err := client.Do(req, &protectedBranches)
		if err != nil {
			return false, errors.Wrapf(err, `failed to get protected branches, page %d`, options.Page)
		}

		if len(protectedBranches) == 0 {
			break
		}

		for _, protectedBranch := range protectedBranches {
			// We rename to be consistent between getting and updating.
			protectedBranch["allowed_to_push"] = protectedBranch["push_access_levels"]
			protectedBranch["allowed_to_merge"] = protectedBranch["merge_access_levels"]
			protectedBranch["allowed_to_unprotect"] = protectedBranch["unprotect_access_levels"]

			// Making sure id is an integer.
			castFloatsToInts(protectedBranch)

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

	// We sort by protected branch's id so that we have deterministic order.
	sort.Slice(configuration.ProtectedBranches, func(i, j int) bool {
		return configuration.ProtectedBranches[i]["id"].(int) < configuration.ProtectedBranches[j]["id"].(int)
	})

	return false, nil
}

// parseProtectedBranchesDocumentation parses GitLab's documentation in Markdown for
// protected branches API endpoint and extracts description of fields used to describe
// protected branches.
func parseProtectedBranchesDocumentation(input []byte) (map[string]string, errors.E) {
	return parseTable(input, "Update a protected branch", nil)
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
func (c *SetCommand) updateProtectedBranches(client *gitlab.Client, configuration *Configuration) errors.E {
	if configuration.ProtectedBranches == nil {
		return nil
	}

	fmt.Fprintf(os.Stderr, "Updating protected branches...\n")

	options := &gitlab.ListProtectedBranchesOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: maxGitLabPageSize,
			Page:    1,
		},
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

	existingProtectedBranches := map[string]*gitlab.ProtectedBranch{}
	existingProtectedBranchesSet := mapset.NewThreadUnsafeSet()
	for _, protectedBranch := range protectedBranches {
		existingProtectedBranchesSet.Add(protectedBranch.Name)
		existingProtectedBranches[protectedBranch.Name] = protectedBranch
	}

	wantedProtectedBranchesSet := mapset.NewThreadUnsafeSet()
	for i, protectedBranch := range configuration.ProtectedBranches {
		name, ok := protectedBranch["name"]
		if !ok {
			return errors.Errorf(`protected branch in configuration at index %d does not have "name"`, i)
		}
		n, ok := name.(string)
		if !ok {
			return errors.Errorf(`invalid "name" in "protected_branches" at index %d`, i)
		}
		wantedProtectedBranchesSet.Add(n)
	}

	extraProtectedBranchesSet := existingProtectedBranchesSet.Difference(wantedProtectedBranchesSet)
	for _, extraProtectedBranch := range extraProtectedBranchesSet.ToSlice() {
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

		// If project already have this protected branch, we update it.
		// Others are updated if they contain an ID or created new if they do not contain an ID.
		if existingProtectedBranchesSet.Contains(name) {
			// We know it exists.
			existingProtectedBranch := existingProtectedBranches[name]

			// We have to mark any access level which does not exist anymore for deletion.
			for _, ii := range []struct {
				Name         string
				AccessLevels []*gitlab.BranchAccessDescription
			}{
				{"push_access_levels", existingProtectedBranch.PushAccessLevels},
				{"allowed_to_merge", existingProtectedBranch.MergeAccessLevels},
				{"unprotect_access_levels", existingProtectedBranch.UnprotectAccessLevels},
			} {
				existingAccessLevelsSet := mapset.NewThreadUnsafeSet()
				for _, accessLevel := range ii.AccessLevels {
					existingAccessLevelsSet.Add(accessLevel.ID)
				}

				wantedAccessLevels, ok := protectedBranch[ii.Name]
				if ok {
					levels, ok := wantedAccessLevels.([]map[string]interface{})
					if !ok {
						return errors.Errorf(`invalid access level in "%s" for protected branch "%s"`, ii.Name, name)
					}
					for _, level := range levels {
						id, ok := level["id"]
						if ok && !existingAccessLevelsSet.Contains(id.(int)) {
							// We mark any access level with ID which does not exist among
							// existing access levels for deletion (destroy).
							level["_destroy"] = true
						}
					}
				}
			}

			req, err := client.NewRequest(http.MethodPatch, u, protectedBranch, nil)
			if err != nil {
				return errors.Wrapf(err, `failed to update protected branch "%s"`, name)
			}
			_, err = client.Do(req, nil)
			if err != nil {
				return errors.Wrapf(err, `failed to update protected branch "%s"`, name)
			}
		} else {
			// We create a new protected branch.
			req, err := client.NewRequest(http.MethodPost, u, protectedBranch, nil)
			if err != nil {
				return errors.Wrapf(err, `failed to protect branch "%s"`, name)
			}
			_, err = client.Do(req, nil)
			if err != nil {
				return errors.Wrapf(err, `failed to protect branch "%s"`, name)
			}
		}
	}

	return nil
}
