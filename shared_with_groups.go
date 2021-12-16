package config

import (
	"fmt"
	"net/http"

	mapset "github.com/deckarep/golang-set"
	"github.com/xanzy/go-gitlab"
	"gitlab.com/tozd/go/errors"
)

// getSharedWithGroupsDescriptions obtains description of fields used to describe payload for
// sharing a project with a group from GitLab's documentation for projects API endpoint.
func getSharedWithGroupsDescriptions() (map[string]string, errors.E) {
	data, err := downloadFile("https://gitlab.com/gitlab-org/gitlab/-/raw/master/doc/api/projects.md")
	if err != nil {
		return nil, errors.Wrap(err, `failed to get share project descriptions`)
	}
	return parseShareDocumentation(data)
}

// updateSharedWithGroups updates GitLab project's sharing with groups using GitLab projects API endpoint
// based on the configuration struct.
//
// It first removes all groups for which the project should not be shared anymore with,
// and then updates or adds groups for which the project should be shared with.
// When updating an existing group it briefly removes the group and readds it with
// new configuration.
func updateSharedWithGroups(client *gitlab.Client, projectID string, configuration *Configuration) errors.E {
	project, _, err := client.Projects.GetProject(projectID, nil)
	if err != nil {
		return errors.Wrap(err, `failed to get project`)
	}

	existingGroups := mapset.NewThreadUnsafeSet()
	for _, group := range project.SharedWithGroups {
		existingGroups.Add(group.GroupID)
	}
	wantedGroups := mapset.NewThreadUnsafeSet()
	for _, group := range configuration.SharedWithGroups {
		wantedGroups.Add(group["group_id"].(int))
	}

	extraGroups := existingGroups.Difference(wantedGroups)
	for _, extraGroup := range extraGroups.ToSlice() {
		groupID := extraGroup.(int) //nolint:errcheck
		_, err := client.Projects.DeleteSharedProjectFromGroup(projectID, groupID)
		if err != nil {
			return errors.Wrapf(err, `failed to unshare group %d`, groupID)
		}
	}

	u := fmt.Sprintf("projects/%s/share", gitlab.PathEscape(projectID))

	for i, group := range configuration.SharedWithGroups {
		groupID, ok := group["group_id"].(int)
		if !ok {
			return errors.Errorf(`invalid "id" in "shared_with_groups" at index %d`, i)
		}
		group["group_id"] = groupID

		// If project is already shared with this group, we have to
		// first unshare to be able to update the share.
		if existingGroups.Contains(groupID) {
			_, err := client.Projects.DeleteSharedProjectFromGroup(projectID, groupID)
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
