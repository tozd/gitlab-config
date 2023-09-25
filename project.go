package config

import (
	"fmt"
	"net/http"
	"os"

	"github.com/xanzy/go-gitlab"
	"gitlab.com/tozd/go/errors"
)

// getProject populates configuration struct with configuration available
// from GitLab projects API endpoint.
func (c *GetCommand) getProject(client *gitlab.Client, configuration *Configuration) (bool, errors.E) {
	fmt.Fprintf(os.Stderr, "Getting project...\n")

	descriptions, errE := getProjectDescriptions(c.DocsRef)
	if errE != nil {
		return false, errE
	}

	u := fmt.Sprintf("projects/%s", gitlab.PathEscape(c.Project))

	req, err := client.NewRequest(http.MethodGet, u, nil, nil)
	if err != nil {
		return false, errors.WithMessage(err, `failed to get project`)
	}

	project := map[string]interface{}{}

	_, err = client.Do(req, &project)
	if err != nil {
		return false, errors.WithMessage(err, `failed to get project`)
	}

	hasSensitive := false

	// We use a separate top-level configuration for avatar instead.
	s, errE := c.getAvatar(client, project, configuration)
	if errE != nil {
		return false, errE
	}
	hasSensitive = hasSensitive || s

	// We use a separate top-level configuration for shared with groups instead.
	s, errE = c.getSharedWithGroups(client, project, configuration)
	if errE != nil {
		return false, errE
	}
	hasSensitive = hasSensitive || s

	// We use a separate top-level configuration for fork relationship.
	s, errE = c.getForkedFromProject(client, project, configuration)
	if errE != nil {
		return false, errE
	}
	hasSensitive = hasSensitive || s

	// Only retain those keys which can be edited through the API
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
			return false, errors.New(`invalid "container_expiration_policy"`)
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
	describeKeys(project, descriptions)

	configuration.Project = project

	return hasSensitive, nil
}

// parseProjectDocumentation parses GitLab's documentation in Markdown for
// projects API endpoint and extracts description of fields used to describe
// an individual project.
func parseProjectDocumentation(input []byte) (map[string]string, errors.E) {
	return parseTable(input, "Edit project", func(key string) string {
		switch key {
		case "public_builds":
			// "public_jobs" is used in get,
			// while "public_builds" is used in edit.
			// See: https://gitlab.com/gitlab-org/gitlab/-/issues/329725
			return "public_jobs"
		case "container_expiration_policy_attributes":
			// "container_expiration_policy" is used in get,
			// while "container_expiration_policy_attributes" is used in edit.
			return "container_expiration_policy"
		case "show_default_award_emojis":
			// Currently it does not work.
			// See: https://gitlab.com/gitlab-org/gitlab/-/issues/348365
			return ""
		case "name", "visibility":
			// Only owners can have "name" and "visibility" fields present in edit
			// project API request, otherwise GitLab returns 403, but we want it
			// to work for maintainers as well. One can include these fields
			// manually into project configuration and it will work for owners.
			return ""
		case "path":
			// If "path" is included in the request, the request does not
			// do anything, even for the owner.
			// See: https://gitlab.com/gitlab-org/gitlab/-/issues/348635
			return ""
		default:
			return key
		}
	})
}

// getProjectDescriptions obtains description of fields used to describe
// an individual project from GitLab's documentation for projects API endpoint.
func getProjectDescriptions(gitRef string) (map[string]string, errors.E) {
	data, err := downloadFile(fmt.Sprintf("https://gitlab.com/gitlab-org/gitlab/-/raw/%s/doc/api/projects.md", gitRef))
	if err != nil {
		return nil, errors.WithMessage(err, "failed to get project configuration descriptions")
	}
	return parseProjectDocumentation(data)
}

// updateProject updates GitLab project's configuration using GitLab projects API endpoint
// based on the configuration struct.
func (c *SetCommand) updateProject(client *gitlab.Client, configuration *Configuration) errors.E {
	if configuration.Project == nil {
		return nil
	}

	fmt.Fprintf(os.Stderr, "Updating project...\n")

	u := fmt.Sprintf("projects/%s", gitlab.PathEscape(c.Project))

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
		return errors.WithMessage(err, "failed to update GitLab project")
	}
	_, err = client.Do(req, nil)
	if err != nil {
		return errors.WithMessage(err, "failed to update GitLab project")
	}

	return nil
}
