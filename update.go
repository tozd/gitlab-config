package config

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/alecthomas/kong"
	"github.com/xanzy/go-gitlab"
	"gitlab.com/tozd/go/errors"
	"gopkg.in/yaml.v3"
)

// We do not use type=path for Input because we want a relative path.
type UpdateCommand struct {
	GitLab

	Input string `short:"i" placeholder:"PATH" default:".gitlab-conf.yml" help:"Where to load the configuration from. Can be \"-\" for stdin. Default is \"${default}\"."`
}

func updateProjectConfig(client *gitlab.Client, projectID string, configuration *Configuration) errors.E {
	u := fmt.Sprintf("projects/%s", pathEscape(projectID))

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

func (c *UpdateCommand) Run(globals *Globals) errors.E {
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

	return nil
}
