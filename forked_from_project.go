package config

import (
	"github.com/xanzy/go-gitlab"
	"gitlab.com/tozd/go/errors"
)

// updateForkedFromProject updates GitLab project's fork relation using GitLab projects API endpoint
// based on the configuration struct.
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
