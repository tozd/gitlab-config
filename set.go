package config

import (
	"fmt"
	"io"
	"os"
	"reflect"

	"github.com/alecthomas/kong"
	"github.com/tozd/sops/v3"
	"github.com/tozd/sops/v3/decrypt"
	"github.com/xanzy/go-gitlab"
	"gitlab.com/tozd/go/errors"
	"gitlab.com/tozd/go/x"
	"gopkg.in/yaml.v3"
)

// We do not use type=path for Input because we want a relative path.

// SetCommand describes parameters for the set command.
type SetCommand struct {
	GitLab

	Input     string `default:".gitlab-conf.yml"                                                          help:"Where to load the configuration from. Can be \"-\" for stdin. Default is \"${default}\"." placeholder:"PATH" short:"i"` //nolint:lll
	EncSuffix string `help:"Remove the suffix from field names before calling APIs. Disabled by default." short:"S"`                                                                                                                    //nolint:lll
	NoDecrypt bool   `help:"Do not attempt to decrypt the configuration."`
}

// Run runs the set command.
func (c *SetCommand) Run(globals *Globals) errors.E {
	if c.Project == "" {
		projectID, errE := x.InferGitLabProjectID(".")
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

	if !c.NoDecrypt {
		decryptedInput, err := decrypt.Data(input, "yaml") //nolint:govet
		if err == nil {
			input = decryptedInput
		} else if !errors.Is(err, sops.MetadataNotFound) {
			if userErr, ok := err.(sops.UserError); ok {
				err = fmt.Errorf("%s\n\n%s", err, userErr.UserError())
			}
			return errors.Wrapf(err, `cannot decrypt configuration from "%s"`, c.Input)
		}
	}

	var configuration Configuration
	err = yaml.Unmarshal(input, &configuration)
	if err != nil {
		return errors.Wrapf(err, `cannot unmarshal configuration from "%s"`, c.Input)
	}

	// We use reflect to go over all struct's fields so we do not have to
	// change this code as Configuration struct evolves.
	v := reflect.ValueOf(configuration)
	for i := 0; i < v.NumField(); i++ {
		removeFieldSuffix(v.Field(i), c.EncSuffix)
	}

	client, err := gitlab.NewClient(c.Token, gitlab.WithBaseURL(c.BaseURL))
	if err != nil {
		return errors.Wrap(err, `failed to create GitLab API client instance`)
	}

	errE := c.updateProject(client, &configuration)
	if errE != nil {
		return errE
	}

	errE = c.updateAvatar(client, &configuration)
	if errE != nil {
		return errE
	}

	errE = c.updateSharedWithGroups(client, &configuration)
	if errE != nil {
		return errE
	}

	errE = c.updateForkedFromProject(client, &configuration)
	if errE != nil {
		return errE
	}

	errE = c.updateLabels(client, &configuration)
	if errE != nil {
		return errE
	}

	errE = c.updateProtectedBranches(client, &configuration)
	if errE != nil {
		return errE
	}

	errE = c.updateVariables(client, &configuration)
	if errE != nil {
		return errE
	}

	fmt.Fprintf(os.Stderr, "Updated everything.\n")

	return nil
}
