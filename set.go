package config

import (
	"io"
	"os"

	"github.com/alecthomas/kong"
	"github.com/xanzy/go-gitlab"
	"gitlab.com/tozd/go/errors"
	"gopkg.in/yaml.v3"
)

// We do not use type=path for Input because we want a relative path.

// SetCommand describes parameters for the set command.
type SetCommand struct {
	GitLab

	Input string `short:"i" placeholder:"PATH" default:".gitlab-conf.yml" help:"Where to load the configuration from. Can be \"-\" for stdin. Default is \"${default}\"."` //nolint:lll
}

// Run runs the set command.
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

	var configuration Configuration
	err = yaml.Unmarshal(input, &configuration)
	if err != nil {
		return errors.Wrapf(err, `cannot unmarshal configuration from "%s"`, c.Input)
	}

	client, err := gitlab.NewClient(c.Token, gitlab.WithBaseURL(c.BaseURL))
	if err != nil {
		return errors.Wrap(err, `failed to create GitLab API client instance`)
	}

	errE := updateProject(client, c.Project, &configuration)
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

	errE = updateProtectedBranches(client, c.Project, &configuration)
	if errE != nil {
		return errE
	}

	return nil
}
