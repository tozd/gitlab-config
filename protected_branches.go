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

// getProtectedBranches populates configuration struct with configuration available
// from GitLab protected branches API endpoint.
func (c *GetCommand) getProtectedBranches(client *gitlab.Client, configuration *Configuration) (bool, errors.E) { //nolint:unparam
	fmt.Fprintf(os.Stderr, "Getting protected branches...\n")

	configuration.ProtectedBranches = []map[string]interface{}{}

	descriptions, errE := getProtectedBranchesDescriptions(c.DocsRef)
	if errE != nil {
		return false, errE
	}
	// We need "name" later on.
	if _, ok := descriptions["name"]; !ok {
		return false, errors.New(`"name" missing in protected branches descriptions`)
	}
	configuration.ProtectedBranchesComment = formatDescriptions(descriptions)

	u := fmt.Sprintf("projects/%s/protected_branches", gitlab.PathEscape(c.Project))
	options := &gitlab.ListProtectedBranchesOptions{ //nolint:exhaustruct
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

			// Making sure ids and levels are an integer.
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

			name, ok := protectedBranch["name"]
			if !ok {
				return false, errors.Errorf(`protected branch is missing "name"`)
			}
			_, ok = name.(string)
			if !ok {
				return false, errors.Errorf(`protected branch "name" is not an string, but %T: %s`, name, name)
			}

			configuration.ProtectedBranches = append(configuration.ProtectedBranches, protectedBranch)
		}

		if response.NextPage == 0 {
			break
		}

		options.Page = response.NextPage
	}

	// We sort by protected branch's name so that we have deterministic order.
	sort.Slice(configuration.ProtectedBranches, func(i, j int) bool {
		// We checked that name is string above.
		return configuration.ProtectedBranches[i]["name"].(string) < configuration.ProtectedBranches[j]["name"].(string) //nolint:forcetypeassert
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
//
// Access levels without the ID field are matched to existing access labels based on
// their fields. Unmatched access levels are created as new.
func (c *SetCommand) updateProtectedBranches(client *gitlab.Client, configuration *Configuration) errors.E { //nolint:maintidx
	if configuration.ProtectedBranches == nil {
		return nil
	}

	fmt.Fprintf(os.Stderr, "Updating protected branches...\n")

	options := &gitlab.ListProtectedBranchesOptions{ //nolint:exhaustruct
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
	existingProtectedBranchesSet := mapset.NewThreadUnsafeSet[string]()
	for _, protectedBranch := range protectedBranches {
		existingProtectedBranchesSet.Add(protectedBranch.Name)
		existingProtectedBranches[protectedBranch.Name] = protectedBranch
	}

	wantedProtectedBranchesSet := mapset.NewThreadUnsafeSet[string]()
	for i, protectedBranch := range configuration.ProtectedBranches {
		name, ok := protectedBranch["name"]
		if !ok {
			return errors.Errorf(`protected branch in configuration at index %d does not have "name"`, i)
		}
		n, ok := name.(string)
		if !ok {
			return errors.Errorf(`protected branch "name" at index %d is not a string, but %T: %s`, i, name, name)
		}
		wantedProtectedBranchesSet.Add(n)
	}

	extraProtectedBranchesSet := existingProtectedBranchesSet.Difference(wantedProtectedBranchesSet)
	for _, protectedBranchName := range extraProtectedBranchesSet.ToSlice() {
		_, err := client.ProtectedBranches.UnprotectRepositoryBranches(c.Project, protectedBranchName)
		if err != nil {
			return errors.Wrapf(err, `failed to unprotect branch "%s"`, protectedBranchName)
		}
	}

	for _, protectedBranch := range configuration.ProtectedBranches {
		// We made sure above that all protected branches in configuration have a string name.
		name := protectedBranch["name"].(string) //nolint:errcheck,forcetypeassert

		// If project already have this protected branch, we update it.
		// Others are updated if they contain an ID or created new if they do not contain an ID.
		if existingProtectedBranchesSet.Contains(name) { //nolint:nestif
			// We know it exists.
			existingProtectedBranch := existingProtectedBranches[name]

			// We have to mark any access level which does not exist anymore for deletion.
			for _, ii := range []struct {
				Name         string
				AccessLevels []*gitlab.BranchAccessDescription
			}{
				{"allowed_to_push", existingProtectedBranch.PushAccessLevels},
				{"allowed_to_merge", existingProtectedBranch.MergeAccessLevels},
				{"allowed_to_unprotect", existingProtectedBranch.UnprotectAccessLevels},
			} {
				existingAccessLevelsSet := mapset.NewThreadUnsafeSet[int]()
				accessLevelToIDs := map[int]int{}
				userIDtoIDs := map[int]int{}
				groupIDtoIDs := map[int]int{}
				for _, accessLevel := range ii.AccessLevels {
					if accessLevel.AccessLevel != 0 {
						accessLevelToIDs[int(accessLevel.AccessLevel)] = accessLevel.ID
					}
					if accessLevel.UserID != 0 {
						userIDtoIDs[accessLevel.UserID] = accessLevel.ID
					}
					if accessLevel.GroupID != 0 {
						groupIDtoIDs[accessLevel.GroupID] = accessLevel.ID
					}
					existingAccessLevelsSet.Add(accessLevel.ID)
				}

				wantedAccessLevels, ok := protectedBranch[ii.Name]
				if !ok {
					wantedAccessLevels = []interface{}{}
				}

				levels, ok := wantedAccessLevels.([]interface{})
				if !ok {
					return errors.Errorf(`invalid access levels "%s" for protected branch "%s"`, ii.Name, name)
				}

				// Set access level IDs if a matching existing access level can be found.
				for i, level := range levels {
					l, ok := level.(map[string]interface{})
					if !ok {
						return errors.Errorf(`invalid access level "%s" at index %d for protected branch "%s"`, ii.Name, i, name)
					}

					// Is access level ID already set?
					id, ok := l["id"]
					if ok {
						// If ID is provided, the access level should exist.
						iid, ok := id.(int) //nolint:govet
						if !ok {
							return errors.Errorf(`access level "%s" "id" at index %d for protected branch "%s" is not an integer, but %T: %s`, ii.Name, i, name, id, id)
						}
						if existingAccessLevelsSet.Contains(iid) {
							continue
						}
						// Access level does not exist with that ID. We remove the ID and leave to matching to
						// find the correct ID, if it exists. Otherwise we will just create a new access level.
						delete(l, "id")
					}

					accessLevel, ok := l["access_level"]
					if ok {
						a, ok := accessLevel.(int) //nolint:govet
						if ok {
							id, ok = accessLevelToIDs[a]
							if ok {
								l["id"] = id
							}
						}
					}
					userID, ok := l["user_id"]
					if ok {
						u, ok := userID.(int) //nolint:govet
						if ok {
							id, ok = userIDtoIDs[u]
							if ok {
								l["id"] = id
							}
						}
					}
					groupID, ok := l["group_id"]
					if ok {
						g, ok := groupID.(int)
						if ok {
							id, ok = groupIDtoIDs[g]
							if ok {
								l["id"] = id
							}
						}
					}
				}

				wantedAccessLevelsSet := mapset.NewThreadUnsafeSet[int]()
				for _, level := range levels {
					// We know it has to be a map.
					id, ok := level.(map[string]interface{})["id"]
					if ok {
						// We checked that id is int above.
						wantedAccessLevelsSet.Add(id.(int)) //nolint:forcetypeassert
					}
				}

				extraAccessLevelsSet := existingAccessLevelsSet.Difference(wantedAccessLevelsSet)
				for _, accessLevelID := range extraAccessLevelsSet.ToSlice() {
					protectedBranch[ii.Name] = append(levels, map[string]interface{}{
						"id":       accessLevelID,
						"_destroy": true,
					})
				}
			}

			req, err := client.NewRequest(http.MethodPatch, fmt.Sprintf("projects/%s/protected_branches/%s", gitlab.PathEscape(c.Project), name), protectedBranch, nil)
			if err != nil {
				return errors.Wrapf(err, `failed to update protected branch "%s"`, name)
			}
			_, err = client.Do(req, nil)
			if err != nil {
				return errors.Wrapf(err, `failed to update protected branch "%s"`, name)
			}
		} else {
			// We create a new protected branch.
			req, err := client.NewRequest(http.MethodPost, fmt.Sprintf("projects/%s/protected_branches", gitlab.PathEscape(c.Project)), protectedBranch, nil)
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
