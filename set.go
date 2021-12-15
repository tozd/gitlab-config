package config

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/alecthomas/kong"
	mapset "github.com/deckarep/golang-set"
	"github.com/xanzy/go-gitlab"
	"gitlab.com/tozd/go/errors"
	"gopkg.in/yaml.v3"
)

// We do not use type=path for Input because we want a relative path.
type SetCommand struct {
	GitLab

	Input string `short:"i" placeholder:"PATH" default:".gitlab-conf.yml" help:"Where to load the configuration from. Can be \"-\" for stdin. Default is \"${default}\"."`
}

func updateProjectConfig(client *gitlab.Client, projectID string, configuration *Configuration) errors.E {
	u := fmt.Sprintf("projects/%s", gitlab.PathEscape(projectID))

	// For now we provide both keys, the new and the deprecated.
	containerExpirationPolicy, ok := configuration.Project["container_expiration_policy"]
	if ok {
		if containerExpirationPolicy != nil {
			containerExpirationPolicy := containerExpirationPolicy.(map[string]interface{})
			containerExpirationPolicy["name_regex"] = containerExpirationPolicy["name_regex_delete"]
			configuration.Project["container_expiration_policy"] = containerExpirationPolicy
		}

		// We have to rename the key to what is used in edit.
		configuration.Project["container_expiration_policy_attributes"] = configuration.Project["container_expiration_policy"]
		delete(configuration.Project, "container_expiration_policy")
	}

	// We have to rename the key to what is used in edit.
	publicJobs, ok := configuration.Project["public_jobs"]
	if ok {
		configuration.Project["public_builds"] = publicJobs
		delete(configuration.Project, "public_jobs")
	}

	req, err := client.NewRequest(http.MethodPut, u, configuration.Project, nil)
	if err != nil {
		return errors.Wrap(err, `failed to update GitLab project`)
	}

	_, err = client.Do(req, nil)
	if err != nil {
		return errors.Wrap(err, `failed to update GitLab project`)
	}

	return nil
}

func updateAvatar(client *gitlab.Client, projectID string, configuration *Configuration) errors.E {
	if configuration.Avatar == "" {
		u := fmt.Sprintf("projects/%s", gitlab.PathEscape(projectID))

		// TODO: Make it really remove the avatar.
		//       See: https://gitlab.com/gitlab-org/gitlab/-/issues/348498
		req, err := client.NewRequest(http.MethodPut, u, map[string]interface{}{"avatar": nil}, nil)
		if err != nil {
			return errors.Wrap(err, `failed to delete GitLab project avatar`)
		}

		_, err = client.Do(req, nil)
		if err != nil {
			return errors.Wrap(err, `failed to delete GitLab project avatar`)
		}
	} else {
		file, err := os.Open(configuration.Avatar)
		if err != nil {
			return errors.Wrapf(err, `failed to open GitLab project avatar file "%s"`, configuration.Avatar)
		}
		defer file.Close()
		_, filename := filepath.Split(configuration.Avatar)
		_, _, err = client.Projects.UploadAvatar(projectID, file, filename)
		if err != nil {
			return errors.Wrap(err, `failed to upload GitLab project avatar`)
		}
	}

	return nil
}

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
		groupID := extraGroup.(int)
		_, err := client.Projects.DeleteSharedProjectFromGroup(projectID, groupID)
		if err != nil {
			return errors.Wrapf(err, `failed to unshare group %d`, groupID)
		}
	}

	u := fmt.Sprintf("projects/%s/share", gitlab.PathEscape(projectID))

	for _, group := range configuration.SharedWithGroups {
		groupID := group["group_id"].(int)
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

func updateForkedFromProject(client *gitlab.Client, projectID string, configuration *Configuration) errors.E {
	project, _, err := client.Projects.GetProject(projectID, nil)
	if err != nil {
		return errors.Wrap(err, `failed to get project`)
	}

	if configuration.ForkedFromProject == 0 {
		if project.ForkedFromProject != nil {
			_, err := client.Projects.DeleteProjectForkRelation(projectID)
			if err != nil {
				return errors.Wrap(err, `failed to delete fork relation`)
			}
		}
	} else if project.ForkedFromProject == nil {
		_, _, err := client.Projects.CreateProjectForkRelation(projectID, configuration.ForkedFromProject)
		if err != nil {
			return errors.Wrapf(err, `failed to create fork relation to project %d`, configuration.ForkedFromProject)
		}
	} else if project.ForkedFromProject.ID != configuration.ForkedFromProject {
		_, err := client.Projects.DeleteProjectForkRelation(projectID)
		if err != nil {
			return errors.Wrap(err, `failed to delete fork relation before creating new`)
		}
		_, _, err = client.Projects.CreateProjectForkRelation(projectID, configuration.ForkedFromProject)
		if err != nil {
			return errors.Wrapf(err, `failed to create fork relation to project %d`, configuration.ForkedFromProject)
		}
	}
	return nil
}

// Labels without the ID field are matched to existing labels based on the name.
// Unmatched labels are created as new. Save configuration with label IDs to be able
// to rename existing labels.
func updateLabels(client *gitlab.Client, projectID string, configuration *Configuration) errors.E {
	options := &gitlab.ListLabelsOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: maxGitLabPageSize,
			Page:    1,
		},
		IncludeAncestorGroups: gitlab.Bool(false),
	}

	labels := []*gitlab.Label{}

	for {
		ls, response, err := client.Labels.ListLabels(projectID, options)
		if err != nil {
			return errors.Wrapf(err, `failed to get project labels, page %d`, options.Page)
		}

		labels = append(labels, ls...)

		if response.NextPage == 0 {
			break
		}

		options.Page = response.NextPage
	}

	existingLabels := mapset.NewThreadUnsafeSet()
	namesToIDs := map[string]int{}
	for _, label := range labels {
		namesToIDs[label.Name] = label.ID
		existingLabels.Add(label.ID)
	}

	// Set label IDs if a matching existing label can be found.
	for i, label := range configuration.Labels {
		// Is label ID already set?
		id, ok := label["id"]
		if ok {
			// If ID is provided, the label should exist.
			id := id.(int)
			if !existingLabels.Contains(id) {
				return errors.Errorf(`label in configuration with ID %d does not exist`, id)
			}
			continue
		}

		name, ok := label["name"]
		if !ok {
			return errors.Errorf(`label in configuration at index %d does not have a name`, i)
		}
		id, ok = namesToIDs[name.(string)]
		if ok {
			label["id"] = id
		}
	}

	wantedLabels := mapset.NewThreadUnsafeSet()
	for _, label := range configuration.Labels {
		id, ok := label["id"]
		if ok {
			wantedLabels.Add(id.(int))
		}
	}

	extraLabels := existingLabels.Difference(wantedLabels)
	for _, extraLabel := range extraLabels.ToSlice() {
		labelID := extraLabel.(int)
		// TODO: Use go-gitlab's function once it is updated to new API.
		//       See: https://github.com/xanzy/go-gitlab/issues/1321
		u := fmt.Sprintf("projects/%s/labels/%d", gitlab.PathEscape(projectID), labelID)
		req, err := client.NewRequest(http.MethodDelete, u, nil, nil)
		if err != nil {
			return errors.Wrapf(err, `failed to delete label %d`, labelID)
		}
		_, err = client.Do(req, nil)
		if err != nil {
			return errors.Wrapf(err, `failed to delete label %d`, labelID)
		}
	}

	for _, label := range configuration.Labels {
		id, ok := label["id"]
		if !ok {
			u := fmt.Sprintf("projects/%s/labels", gitlab.PathEscape(projectID))
			req, err := client.NewRequest(http.MethodPost, u, label, nil)
			if err != nil {
				// We made sure above that all labels in configuration without label ID have name.
				return errors.Wrapf(err, `failed to create label "%s"`, label["name"].(string))
			}
			_, err = client.Do(req, nil)
			if err != nil {
				// We made sure above that all labels in configuration without label ID have name.
				return errors.Wrapf(err, `failed to create label "%s"`, label["name"].(string))
			}
		} else {
			// We made sure above that all labels in configuration with label ID exist.
			id := id.(int)
			u := fmt.Sprintf("projects/%s/labels/%d", gitlab.PathEscape(projectID), id)
			req, err := client.NewRequest(http.MethodPut, u, label, nil)
			if err != nil {
				return errors.Wrapf(err, `failed to update label %d`, id)
			}
			_, err = client.Do(req, nil)
			if err != nil {
				return errors.Wrapf(err, `failed to update label "%d`, id)
			}
		}
	}

	return nil
}

func (c *SetCommand) Run(globals *Globals) errors.E {
	if globals.ChangeTo != "" {
		err := os.Chdir(globals.ChangeTo)
		if err != nil {
			return errors.Wrapf(err, `cannot change current working directory to "%s"`, globals.ChangeTo)
		}
	}

	if c.Project == "" {
		projectID, errE := inferProjectID(".")
		if errE != nil {
			return errE
		}
		c.Project = projectID
	}

	var input []byte
	var err error
	if c.Input != "-" {
		input, err = os.ReadFile(kong.ExpandPath(c.Input))
	} else {
		input, err = io.ReadAll(os.Stdin)
	}
	if err != nil {
		return errors.Wrapf(err, `cannot read configuration from "%s"`, c.Input)
	}

	configuration := Configuration{}

	err = yaml.Unmarshal(input, &configuration)
	if err != nil {
		return errors.Wrapf(err, `cannot unmarshal configuration from "%s"`, c.Input)
	}

	client, err := gitlab.NewClient(c.Token, gitlab.WithBaseURL(c.BaseURL))
	if err != nil {
		return errors.Wrap(err, `failed to create GitLab API client instance`)
	}

	errE := updateProjectConfig(client, c.Project, &configuration)
	if errE != nil {
		return errE
	}

	errE = updateAvatar(client, c.Project, &configuration)
	if errE != nil {
		return errE
	}

	errE = updateSharedWithGroups(client, c.Project, &configuration)
	if errE != nil {
		return errE
	}

	errE = updateForkedFromProject(client, c.Project, &configuration)
	if errE != nil {
		return errE
	}

	errE = updateLabels(client, c.Project, &configuration)
	if errE != nil {
		return errE
	}

	return nil
}
