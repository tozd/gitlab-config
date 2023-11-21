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
//
//nolint:lll
type SetCommand struct {
	GitLab

	Input     string `default:".gitlab-conf.yml" help:"Where to load the configuration from. Can be \"-\" for stdin. Default is \"${default}\"." placeholder:"PATH" short:"i"`
	EncSuffix string `                           help:"Remove the suffix from field names before calling APIs. Disabled by default."                                short:"S"`
	NoDecrypt bool   `                           help:"Do not attempt to decrypt the configuration."`
}

// Run runs the set command.
func (c *SetCommand) Run(_ *Globals) errors.E {
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
		errE := errors.WithMessage(err, "cannot read configuration")
		errors.Details(errE)["path"] = c.Input
		return errE
	}

	if !c.NoDecrypt {
		decryptedInput, err := decrypt.Data(input, "yaml") //nolint:govet
		if err == nil {
			input = decryptedInput
		} else if !errors.Is(err, sops.MetadataNotFound) {
			var userErr sops.UserError
			if errors.As(err, &userErr) {
				err = errors.Errorf("%w\n\n%s", err, userErr.UserError())
			}
			errE := errors.WithMessage(err, "cannot decrypt configuration")
			errors.Details(errE)["path"] = c.Input
			return errE
		}
	}

	var configuration Configuration
	err = yaml.Unmarshal(input, &configuration)
	if err != nil {
		errE := errors.WithMessage(err, "cannot unmarshal configuration")
		errors.Details(errE)["path"] = c.Input
		return errE
	}

	// We use reflect to go over all struct's fields so we do not have to
	// change this code as Configuration struct evolves.
	v := reflect.ValueOf(configuration)
	for i := 0; i < v.NumField(); i++ {
		removeFieldSuffix(v.Field(i), c.EncSuffix)
	}

	client, err := gitlab.NewClient(c.Token, gitlab.WithBaseURL(c.BaseURL))
	if err != nil {
		return errors.WithMessage(err, "failed to create GitLab API client instance")
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

	errE = c.updateApprovals(client, &configuration)
	if errE != nil {
		return errE
	}

	errE = c.updateApprovalRules(client, &configuration)
	if errE != nil {
		return errE
	}

	errE = c.updatePushRules(client, &configuration)
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

	errE = c.updateProtectedTags(client, &configuration)
	if errE != nil {
		return errE
	}

	errE = c.updateVariables(client, &configuration)
	if errE != nil {
		return errE
	}

	errE = c.updatePipelineSchedules(client, &configuration)
	if errE != nil {
		return errE
	}

	fmt.Fprintf(os.Stderr, "Updated everything.\n")

	return nil
}
