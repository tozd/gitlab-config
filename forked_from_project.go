package config

import (
	"fmt"
	"os"

	"github.com/xanzy/go-gitlab"
	"gitlab.com/tozd/go/errors"
)

// getForkedFromProject populates configuration struct with GitLab's project fork relation
// available from GitLab projects API endpoint.
func (c *GetCommand) getForkedFromProject(
	_ *gitlab.Client, project map[string]interface{}, configuration *Configuration,
) (bool, errors.E) { //nolint:unparam
	fmt.Fprintf(os.Stderr, "Getting project fork relation...\n")

	forkedFromProject, ok := project["forked_from_project"]
	if ok && forkedFromProject != nil {
		forkedFromProject, ok := forkedFromProject.(map[string]interface{})
		if !ok {
			return false, errors.New(`invalid "forked_from_project"`)
		}
		forkIDAny, ok := forkedFromProject["id"]
		if !ok {
			return false, errors.New(`"forked_from_project" is missing field "id"`)
		}
		forkIDFloat, ok := forkIDAny.(float64)
		if !ok {
			errE := errors.New(`"forked_from_project"'s field "id" is not a float`)
			errors.Details(errE)["type"] = fmt.Sprintf("%T", forkIDAny)
			errors.Details(errE)["value"] = forkIDAny
			return false, errE
		}
		// Making sure it is an integer.
		forkID := int(forkIDFloat)
		configuration.ForkedFromProject = &forkID
		forkPathWithNamespace := forkedFromProject["path_with_namespace"]
		if forkPathWithNamespace != nil {
			configuration.ForkedFromProjectComment, ok = forkPathWithNamespace.(string)
			if !ok {
				errE := errors.New(`"forked_from_project"'s field "path_with_namespace" is not a string`)
				errors.Details(errE)["type"] = fmt.Sprintf("%T", forkPathWithNamespace)
				errors.Details(errE)["value"] = forkPathWithNamespace
				return false, errE
			}
		}
	} else {
		noProject := 0
		configuration.ForkedFromProject = &noProject
	}

	return false, nil
}

// updateForkedFromProject updates GitLab project's fork relation using GitLab projects API endpoint
// based on the configuration struct.
func (c *SetCommand) updateForkedFromProject(client *gitlab.Client, configuration *Configuration) errors.E {
	if configuration.ForkedFromProject == nil {
		return nil
	}

	fmt.Fprintf(os.Stderr, "Updating project fork relation...\n")

	project, _, err := client.Projects.GetProject(c.Project, nil)
	if err != nil {
		return errors.WithMessage(err, "failed to get project")
	}

	if *configuration.ForkedFromProject == 0 {
		if project.ForkedFromProject != nil {
			_, err := client.Projects.DeleteProjectForkRelation(c.Project)
			if err != nil {
				return errors.WithMessage(err, "failed to delete fork relation")
			}
		}
	} else if project.ForkedFromProject == nil {
		_, _, err := client.Projects.CreateProjectForkRelation(c.Project, *configuration.ForkedFromProject)
		if err != nil {
			errE := errors.WithMessage(err, "failed to create fork relation")
			errors.Details(errE)["to"] = *configuration.ForkedFromProject
			return errE
		}
	} else if project.ForkedFromProject.ID != *configuration.ForkedFromProject {
		_, err := client.Projects.DeleteProjectForkRelation(c.Project)
		if err != nil {
			return errors.WithMessage(err, "failed to delete fork relation before creating new")
		}
		_, _, err = client.Projects.CreateProjectForkRelation(c.Project, *configuration.ForkedFromProject)
		if err != nil {
			errE := errors.WithMessage(err, "failed to create fork relation")
			errors.Details(errE)["to"] = *configuration.ForkedFromProject
			return errE
		}
	}
	return nil
}
