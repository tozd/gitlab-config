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

// getProtectedTags populates configuration struct with configuration available
// from GitLab protected tags API endpoint.
func (c *GetCommand) getProtectedTags(client *gitlab.Client, configuration *Configuration) (bool, errors.E) {
	fmt.Fprintf(os.Stderr, "Getting protected tags...\n")

	configuration.ProtectedTags = []map[string]interface{}{}

	descriptions, errE := getProtectedTagsDescriptions(c.DocsRef)
	if errE != nil {
		return false, errE
	}
	configuration.ProtectedTagsComment = formatDescriptions(descriptions)

	u := fmt.Sprintf("projects/%s/protected_tags", gitlab.PathEscape(c.Project))
	options := &gitlab.ListProtectedTagsOptions{
		PerPage: maxGitLabPageSize,
		Page:    1,
	}

	for {
		req, err := client.NewRequest(http.MethodGet, u, options, nil)
		if err != nil {
			return false, errors.Wrapf(err, `failed to get protected tags, page %d`, options.Page)
		}

		protectedTags := []map[string]interface{}{}

		response, err := client.Do(req, &protectedTags)
		if err != nil {
			return false, errors.Wrapf(err, `failed to get protected tags, page %d`, options.Page)
		}

		if len(protectedTags) == 0 {
			break
		}

		for _, protectedTag := range protectedTags {
			// We rename to be consistent between getting and updating.
			protectedTag["allowed_to_create"] = protectedTag["create_access_levels"]

			// Making sure ids and levels are an integer.
			castFloatsToInts(protectedTag)

			// Only retain those keys which can be edited through the API
			// (which are those available in descriptions).
			for key := range protectedTag {
				_, ok := descriptions[key]
				if !ok {
					delete(protectedTag, key)
				}
			}

			// Make the description be a comment for the sequence item.
			renameMapField(protectedTag, "access_level_description", "comment:")

			configuration.ProtectedTags = append(configuration.ProtectedTags, protectedTag)
		}

		if response.NextPage == 0 {
			break
		}

		options.Page = response.NextPage
	}

	// We sort by protected tag's id so that we have deterministic order.
	sort.Slice(configuration.ProtectedTags, func(i, j int) bool {
		return configuration.ProtectedTags[i]["id"].(int) < configuration.ProtectedTags[j]["id"].(int)
	})

	return false, nil
}

// parseProtectedTagsDocumentation parses GitLab's documentation in Markdown for
// protected tags API endpoint and extracts description of fields used to describe
// protected tags.
func parseProtectedTagsDocumentation(input []byte) (map[string]string, errors.E) {
	return parseTable(input, "Protect repository tags", func(key string) string {
		switch key {
		case "create_access_level":
			// We prefer that everything is done through "allowed_to_create".
			return ""
		default:
			return key
		}
	})
}

// getProtectedTagsDescriptions obtains description of fields used to describe
// an individual protected tags from GitLab's documentation for protected tags API endpoint.
func getProtectedTagsDescriptions(gitRef string) (map[string]string, errors.E) {
	data, err := downloadFile(fmt.Sprintf("https://gitlab.com/gitlab-org/gitlab/-/raw/%s/doc/api/protected_tags.md", gitRef))
	if err != nil {
		return nil, errors.Wrap(err, `failed to get protected tags descriptions`)
	}
	return parseProtectedTagsDocumentation(data)
}

// updateProtectedTags updates GitLab project's protected tags using GitLab
// protected tags API endpoint based on the configuration struct.
//
// Access levels without the ID field are matched to existing access labels based on
// their fields. Unmatched access levels are created as new.
func (c *SetCommand) updateProtectedTags(client *gitlab.Client, configuration *Configuration) errors.E {
	if configuration.ProtectedTags == nil {
		return nil
	}

	fmt.Fprintf(os.Stderr, "Updating protected tags...\n")

	options := &gitlab.ListProtectedTagsOptions{
		PerPage: maxGitLabPageSize,
		Page:    1,
	}

	protectedTags := []*gitlab.ProtectedTag{}

	for {
		pt, response, err := client.ProtectedTags.ListProtectedTags(c.Project, options)
		if err != nil {
			return errors.Wrapf(err, `failed to get protected tags, page %d`, options.Page)
		}

		protectedTags = append(protectedTags, pt...)

		if response.NextPage == 0 {
			break
		}

		options.Page = response.NextPage
	}

	existingProtectedTags := map[string]*gitlab.ProtectedTag{}
	existingProtectedTagsSet := mapset.NewThreadUnsafeSet()
	for _, protectedTag := range protectedTags {
		existingProtectedTagsSet.Add(protectedTag.Name)
		existingProtectedTags[protectedTag.Name] = protectedTag
	}

	wantedProtectedTagsSet := mapset.NewThreadUnsafeSet()
	for i, protectedTag := range configuration.ProtectedTags {
		name, ok := protectedTag["name"]
		if !ok {
			return errors.Errorf(`protected tag in configuration at index %d does not have "name"`, i)
		}
		n, ok := name.(string)
		if !ok {
			return errors.Errorf(`invalid "name" in "protected_tags" at index %d`, i)
		}
		wantedProtectedTagsSet.Add(n)
	}

	extraProtectedTagsSet := existingProtectedTagsSet.Difference(wantedProtectedTagsSet)
	for _, extraProtectedTag := range extraProtectedTagsSet.ToSlice() {
		protectedTagName := extraProtectedTag.(string) //nolint:errcheck
		_, err := client.ProtectedTags.UnprotectRepositoryTags(c.Project, protectedTagName)
		if err != nil {
			return errors.Wrapf(err, `failed to unprotect tag "%s"`, protectedTagName)
		}
	}

	for _, protectedTag := range configuration.ProtectedTags {
		// We made sure above that all protected tags in configuration have a string name.
		name := protectedTag["name"].(string) //nolint:errcheck

		// If project already have this protected tag, we update it.
		// Others are updated if they contain an ID or created new if they do not contain an ID.
		if existingProtectedTagsSet.Contains(name) {
			// We know it exists.
			existingProtectedTag := existingProtectedTags[name]

			// We have to mark any access level which does not exist anymore for deletion.
			for _, ii := range []struct {
				Name         string
				AccessLevels []*gitlab.TagAccessDescription
			}{
				{"allowed_to_create", existingProtectedTag.CreateAccessLevels},
			} {
				existingAccessLevelsSet := mapset.NewThreadUnsafeSet()
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

				wantedAccessLevels, ok := protectedTag[ii.Name]
				if !ok {
					wantedAccessLevels = []interface{}{}
				}

				levels, ok := wantedAccessLevels.([]interface{})
				if !ok {
					return errors.Errorf(`invalid access levels "%s" for protected tag "%s"`, ii.Name, name)
				}

				// Set access level IDs if a matching existing access level can be found.
				for i, level := range levels {
					l, ok := level.(map[string]interface{})
					if !ok {
						return errors.Errorf(`invalid access level "%s" at index %d for protected tag "%s"`, ii.Name, i, name)
					}

					// Is access level ID already set?
					id, ok := l["id"]
					if ok {
						// If ID is provided, the access level should exist.
						id, ok := id.(int) //nolint:govet
						if !ok {
							return errors.Errorf(`invalid "id" for access level "%s" at index %d for protected tag "%s"`, ii.Name, i, name)
						}
						if existingAccessLevelsSet.Contains(id) {
							continue
						}
						// Access level does not exist with that ID. We remove the ID and leave to matching to
						// find the correct ID, if it exists. Otherwise we will just create a new access level.
						delete(l, "id")
					}

					accessLevel, ok := l["access_level"]
					if ok {
						a, ok := accessLevel.(int)
						if ok {
							id, ok = accessLevelToIDs[a]
							if ok {
								l["id"] = id
							}
						}
					}
					userID, ok := l["user_id"]
					if ok {
						u, ok := userID.(int)
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

				wantedAccessLevelsSet := mapset.NewThreadUnsafeSet()
				for _, level := range levels {
					// We know it has to be a map.
					id, ok := level.(map[string]interface{})["id"]
					if ok {
						wantedAccessLevelsSet.Add(id.(int))
					}
				}

				extraAccessLevelsSet := existingAccessLevelsSet.Difference(wantedAccessLevelsSet)
				for _, extraAccessLevel := range extraAccessLevelsSet.ToSlice() {
					accessLevelID := extraAccessLevel.(int) //nolint:errcheck

					protectedTag[ii.Name] = append(levels, map[string]interface{}{
						"id":       accessLevelID,
						"_destroy": true,
					})
				}
			}

			req, err := client.NewRequest(http.MethodPatch, fmt.Sprintf("projects/%s/protected_tags/%s", gitlab.PathEscape(c.Project), name), protectedTag, nil)
			if err != nil {
				return errors.Wrapf(err, `failed to update protected tag "%s"`, name)
			}
			_, err = client.Do(req, nil)
			if err != nil {
				return errors.Wrapf(err, `failed to update protected tag "%s"`, name)
			}
		} else {
			// We create a new protected tag.
			req, err := client.NewRequest(http.MethodPost, fmt.Sprintf("projects/%s/protected_tags", gitlab.PathEscape(c.Project)), protectedTag, nil)
			if err != nil {
				return errors.Wrapf(err, `failed to protect tag "%s"`, name)
			}
			_, err = client.Do(req, nil)
			if err != nil {
				return errors.Wrapf(err, `failed to protect tag "%s"`, name)
			}
		}
	}

	return nil
}
