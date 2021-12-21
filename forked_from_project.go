package config

import (
	"fmt"

	"github.com/xanzy/go-gitlab"
	"gitlab.com/tozd/go/errors"
)

// getForkedFromProject populates configuration struct with GitLab's project fork relation
// available from GitLab projects API endpoint.
func getForkedFromProject(
	client *gitlab.Client, project map[string]interface{}, configuration *Configuration,
) errors.E {
	fmt.Printf("Getting project fork relation...\n")

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

	return nil
}

// updateForkedFromProject updates GitLab project's fork relation using GitLab projects API endpoint
// based on the configuration struct.
func updateForkedFromProject(client *gitlab.Client, projectID string, configuration *Configuration) errors.E {
	fmt.Printf("Updating project fork relation...\n")

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
