package config

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/xanzy/go-gitlab"
	"gitlab.com/tozd/go/errors"
)

// getProject populates configuration struct with configuration available
// from GitLab projects API endpoint.
func getProject(client *gitlab.Client, projectID, avatarPath string, configuration *Configuration) errors.E {
	descriptions, errE := getProjectDescriptions()
	if errE != nil {
		return errE
	}

	u := fmt.Sprintf("projects/%s", gitlab.PathEscape(projectID))

	req, err := client.NewRequest(http.MethodGet, u, nil, nil)
	if err != nil {
		return errors.Wrap(err, `failed to get project`)
	}

	project := map[string]interface{}{}

	_, err = client.Do(req, &project)
	if err != nil {
		return errors.Wrap(err, `failed to get project`)
	}

	// We use a separate top-level configuration for avatar instead.
	avatarURL, ok := project["avatar_url"]
	if ok && avatarURL != nil {
		avatarURL, ok := avatarURL.(string) //nolint:govet
		if !ok {
			return errors.New(`invalid "avatar_url"`)
		}
		avatarExt := path.Ext(avatarURL)
		err := checkAvatarExtension(avatarExt)
		if err != nil {
			return errors.Wrapf(err, `invalid "avatar_url": %s`, avatarURL)
		}
		// TODO: Make this work for private avatars, too.
		//       See: https://gitlab.com/gitlab-org/gitlab/-/issues/25498
		avatar, err := downloadFile(avatarURL)
		if err != nil {
			return errors.Wrapf(err, `failed to get project avatar from "%s"`, avatarURL)
		}
		avatarPath = strings.TrimSuffix(avatarPath, path.Ext(avatarPath)) + avatarExt
		err = os.WriteFile(avatarPath, avatar, fileMode)
		if err != nil {
			return errors.Wrapf(err, `failed to save avatar to "%s"`, avatarPath)
		}
		configuration.Avatar = avatarPath
	}

	// We use a separate top-level configuration for shared with groups instead.
	sharedWithGroups, ok := project["shared_with_groups"]
	if ok && sharedWithGroups != nil {
		sharedWithGroups, ok := sharedWithGroups.([]interface{}) //nolint:govet
		if !ok {
			return errors.New(`invalid "shared_with_groups"`)
		}
		if len(sharedWithGroups) > 0 {
			configuration.SharedWithGroups = []map[string]interface{}{}
			shareDescriptions, err := getSharedWithGroupsDescriptions()
			if err != nil {
				return err
			}
			for i, sharedWithGroup := range sharedWithGroups {
				sharedWithGroup, ok := sharedWithGroup.(map[string]interface{})
				if !ok {
					return errors.Errorf(`invalid "shared_with_groups" at index %d`, i)
				}
				groupFullPath := sharedWithGroup["group_full_path"]
				// Rename because share API has a different key than get project API.
				sharedWithGroup["group_access"] = sharedWithGroup["group_access_level"]
				// Making sure it is an integer.
				sharedWithGroup["group_id"] = int(sharedWithGroup["group_id"].(float64))

				// Only retain those keys which can be edited through the share API
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
			configuration.SharedWithGroupsComment = formatDescriptions(shareDescriptions)
		}
	}

	// We use a separate top-level configuration for fork relationship.
	forkedFromProject, ok := project["forked_from_project"]
	if ok && forkedFromProject != nil {
		forkedFromProject, ok := forkedFromProject.(map[string]interface{})
		if !ok {
			return errors.New(`invalid "forked_from_project"`)
		}
		forkID, ok := forkedFromProject["id"]
		if ok {
			// Making sure it is an integer.
			configuration.ForkedFromProject = int(forkID.(float64))
			forkPathWithNamespace := forkedFromProject["path_with_namespace"]
			if forkPathWithNamespace != nil {
				configuration.ForkedFromProjectComment, ok = forkPathWithNamespace.(string)
				if !ok {
					return errors.New(`invalid "path_with_namespace" in "forked_from_project"`)
				}
			}
		}
	}

	// Only retain those keys which can be edited through the edit API
	// (which are those available in descriptions). We cannot add comments
	// at the same time because we might delete them, too, because they are
	// not found in descriptions.
	for key := range project {
		_, ok := descriptions[key]
		if !ok {
			delete(project, key)
		}
	}

	// This cannot be configured simply through the edit API, this just enabled/disables it.
	// We use a separate top-level configuration for mirroring instead.
	delete(project, "mirror")

	// Remove deprecated name_regex key in favor of new name_regex_delete.
	if project["container_expiration_policy"] != nil {
		policy, ok := project["container_expiration_policy"].(map[string]interface{})
		if !ok {
			return errors.New(`invalid "container_expiration_policy"`)
		}
		if policy["name_regex"] != nil && policy["name_regex_delete"] == nil {
			policy["name_regex_delete"] = policy["name_regex"]
			delete(policy, "name_regex")
		} else if policy["name_regex"] != nil && policy["name_regex_delete"] != nil {
			delete(policy, "name_regex")
		}

		// It is not an editable key.
		delete(policy, "next_run_at")
	}

	// Add comments for keys. We process these keys before writing YAML out.
	for key := range project {
		project["comment:"+key] = descriptions[key]
	}

	configuration.Project = project

	return nil
}

// getProjectDescriptions obtains description of fields used to describe
// an individual project from GitLab's documentation for projects API endpoint.
func getProjectDescriptions() (map[string]string, errors.E) {
	data, err := downloadFile("https://gitlab.com/gitlab-org/gitlab/-/raw/master/doc/api/projects.md")
	if err != nil {
		return nil, errors.Wrap(err, `failed to get project configuration descriptions`)
	}
	return parseProjectDocumentation(data)
}

// updateProject updates GitLab project's configuration using GitLab projects API endpoint
// based on the configuration struct.
func updateProject(client *gitlab.Client, projectID string, configuration *Configuration) errors.E {
	u := fmt.Sprintf("projects/%s", gitlab.PathEscape(projectID))

	// For now we provide both keys, the new and the deprecated.
	containerExpirationPolicy, ok := configuration.Project["container_expiration_policy"]
	if ok {
		if containerExpirationPolicy != nil {
			containerExpirationPolicy, ok := containerExpirationPolicy.(map[string]interface{}) //nolint:govet
			if !ok {
				return errors.New(`invalid "container_expiration_policy"`)
			}
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
