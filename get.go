package config

import (
	"os"

	"github.com/xanzy/go-gitlab"
	"gitlab.com/tozd/go/errors"
)

// We do not use type=path for Output because we want a relative path.

// GetCommand describes parameters for the get command.
type GetCommand struct {
	GitLab

	Output string `short:"o" placeholder:"PATH" default:".gitlab-conf.yml" help:"Where to save the configuration to. Can be \"-\" for stdout. Default is \"${default}\"."`        //nolint:lll
	Avatar string `short:"a" placeholder:"PATH" default:".gitlab-avatar.img" help:"Where to save the avatar to. File extension is set automatically. Default is \"${default}\"."` //nolint:lll
}

// Run runs the get command.
func (c *GetCommand) Run(globals *Globals) errors.E {
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

	client, err := gitlab.NewClient(c.Token, gitlab.WithBaseURL(c.BaseURL))
	if err != nil {
		return errors.Wrap(err, `failed to create GitLab API client instance`)
	}

	var configuration Configuration

	errE := getProject(client, c.Project, c.Avatar, &configuration)
	if errE != nil {
		return errE
	}

	errE = getLabels(client, c.Project, &configuration)
	if errE != nil {
		return errE
	}

	errE = getProtectedBranches(client, c.Project, &configuration)
	if errE != nil {
		return errE
	}

	errE = getVariables(client, c.Project, &configuration)
	if errE != nil {
		return errE
	}

	return saveConfiguration(&configuration, c.Output)
}
