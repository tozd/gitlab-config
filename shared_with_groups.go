package config

import (
	"fmt"
	"net/http"
	"os"

	mapset "github.com/deckarep/golang-set"
	"github.com/xanzy/go-gitlab"
	"gitlab.com/tozd/go/errors"
)

// getSharedWithGroups populates configuration struct with GitLab's project's sharing
// with groups available from GitLab projects API endpoint.
func (c *GetCommand) getSharedWithGroups(
	client *gitlab.Client, project map[string]interface{}, configuration *Configuration,
) (bool, errors.E) {
	fmt.Fprintf(os.Stderr, "Getting sharing with groups...\n")

	configuration.SharedWithGroups = []map[string]interface{}{}

	shareDescriptions, err := getSharedWithGroupsDescriptions(c.DocsRef)
	if err != nil {
		return false, err
	}
	configuration.SharedWithGroupsComment = formatDescriptions(shareDescriptions)

	sharedWithGroups, ok := project["shared_with_groups"]
	if ok && sharedWithGroups != nil {
		sharedWithGroups, ok := sharedWithGroups.([]interface{})
		if !ok {
			return false, errors.New(`invalid "shared_with_groups"`)
		}
		for i, sharedWithGroup := range sharedWithGroups {
			sharedWithGroup, ok := sharedWithGroup.(map[string]interface{})
			if !ok {
				return false, errors.Errorf(`invalid "shared_with_groups" at index %d`, i)
			}
			groupFullPath := sharedWithGroup["group_full_path"]
			// Rename because share API has a different key than get project API.
			sharedWithGroup["group_access"] = sharedWithGroup["group_access_level"]
			// Making sure it is an integer.
			sharedWithGroup["group_id"] = int(sharedWithGroup["group_id"].(float64))

			// Only retain those keys which can be edited through the API
			// (which are those available in descriptions).
			for key := range sharedWithGroup {
				_, ok = shareDescriptions[key]
				if !ok {
					delete(sharedWithGroup, key)
				}
			}

			// Add comment for the sequence item itself.
			if groupFullPath != nil {
				sharedWithGroup["comment:"] = groupFullPath
			}

			configuration.SharedWithGroups = append(configuration.SharedWithGroups, sharedWithGroup)
		}
	}

	return false, nil
}

// parseSharedWithGroupsDocumentation parses GitLab's documentation in Markdown for
// projects API endpoint and extracts description of fields used to describe
// payload for sharing a project with a group.
func parseSharedWithGroupsDocumentation(input []byte) (map[string]string, errors.E) {
	return parseTable(input, "Share project with group", nil)
}

// getSharedWithGroupsDescriptions obtains description of fields used to describe payload for
// sharing a project with a group from GitLab's documentation for projects API endpoint.
func getSharedWithGroupsDescriptions(gitRef string) (map[string]string, errors.E) {
	data, err := downloadFile(fmt.Sprintf("https://gitlab.com/gitlab-org/gitlab/-/raw/%s/doc/api/projects.md", gitRef))
	if err != nil {
		return nil, errors.Wrap(err, `failed to get share project descriptions`)
	}
	return parseSharedWithGroupsDocumentation(data)
}

// updateSharedWithGroups updates GitLab project's sharing with groups using GitLab projects API endpoint
// based on the configuration struct.
//
// It first removes all groups for which the project should not be shared anymore with,
// and then updates or adds groups for which the project should be shared with.
// When updating an existing group it briefly removes the group and readds it with
// new configuration.
func (c *SetCommand) updateSharedWithGroups(client *gitlab.Client, configuration *Configuration) errors.E {
	if configuration.SharedWithGroups == nil {
		return nil
	}

	fmt.Fprintf(os.Stderr, "Updating sharing with groups...\n")

	project, _, err := client.Projects.GetProject(c.Project, nil)
	if err != nil {
		return errors.Wrap(err, `failed to get project`)
	}

	existingGroupsSet := mapset.NewThreadUnsafeSet()
	for _, group := range project.SharedWithGroups {
		existingGroupsSet.Add(group.GroupID)
	}
	wantedGroupsSet := mapset.NewThreadUnsafeSet()
	for _, group := range configuration.SharedWithGroups {
		wantedGroupsSet.Add(group["group_id"].(int))
	}

	extraGroupsSet := existingGroupsSet.Difference(wantedGroupsSet)
	for _, extraGroup := range extraGroupsSet.ToSlice() {
		groupID := extraGroup.(int) //nolint:errcheck
		_, err := client.Projects.DeleteSharedProjectFromGroup(c.Project, groupID)
		if err != nil {
			return errors.Wrapf(err, `failed to unshare group %d`, groupID)
		}
	}

	u := fmt.Sprintf("projects/%s/share", gitlab.PathEscape(c.Project))

	for i, group := range configuration.SharedWithGroups {
		groupID, ok := group["group_id"].(int)
		if !ok {
			return errors.Errorf(`invalid "id" in "shared_with_groups" at index %d`, i)
		}
		group["group_id"] = groupID

		// If project is already shared with this group, we have to
		// first unshare to be able to update the share.
		if existingGroupsSet.Contains(groupID) {
			_, err := client.Projects.DeleteSharedProjectFromGroup(c.Project, groupID)
			if err != nil {
				return errors.Wrapf(err, `failed to unshare group %d before resharing`, groupID)
			}
		}

		req, err := client.NewRequest(http.MethodPost, u, group, nil)
		if err != nil {
			return errors.Wrapf(err, `failed to share group %d`, groupID)
		}
		_, err = client.Do(req, nil)
		if err != nil {
			return errors.Wrapf(err, `failed to share group %d`, groupID)
		}
	}

	return nil
}
