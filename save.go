package config

import (
	"bytes"
	"fmt"
	"net/http"
	"os"

	"github.com/alecthomas/kong"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/mitchellh/go-wordwrap"
	"github.com/xanzy/go-gitlab"
	"gitlab.com/tozd/go/errors"
	"gopkg.in/yaml.v3"
)

// We do not use type=path for Output because we want a relative path.
type SaveCommand struct {
	GitLab

	Output string `short:"o" placeholder:"PATH" type:"string" default:".gitlab-conf.yml" help:"Where to save the configuration to. Can be \"-\" for stdout. Default is \"${default}\"."`
}

func getProjectConfig(client *gitlab.Client, projectID string, descriptions map[string]string, configuration *Configuration) errors.E {
	u := fmt.Sprintf("projects/%s", pathEscape(projectID))

	req, err := client.NewRequest(http.MethodGet, u, nil, nil)
	if err != nil {
		return errors.Wrap(err, `failed to get GitLab project`)
	}

	project := map[string]interface{}{}

	_, err = client.Do(req, &project)
	if err != nil {
		return errors.Wrap(err, `failed to get GitLab project`)
	}

	for key := range project {
		_, ok := descriptions[key]
		if !ok {
			delete(project, key)
		}
	}

	delete(project, "mirror")
	if project["container_expiration_policy"] != nil {
		container_expiration_policy := project["container_expiration_policy"].(map[string]interface{})
		if container_expiration_policy["name_regex"] != nil && container_expiration_policy["name_regex_delete"] == nil {
			container_expiration_policy["name_regex_delete"] = container_expiration_policy["name_regex"]
			delete(container_expiration_policy, "name_regex")
		} else if container_expiration_policy["name_regex"] != nil && container_expiration_policy["name_regex_delete"] != nil {
			delete(container_expiration_policy, "name_regex")
		}
	}

	configuration.Project = project

	return nil
}

func downloadFile(url string) ([]byte, errors.E) {
	client, _ := gitlab.NewClient("")

	req, err := retryablehttp.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if client.UserAgent != "" {
		req.Header.Set("User-Agent", client.UserAgent)
	}

	buffer := bytes.Buffer{}

	_, err = client.Do(req, &buffer)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return buffer.Bytes(), nil
}

func getProjectConfigDescriptions() (map[string]string, errors.E) {
	data, err := downloadFile("https://gitlab.com/gitlab-org/gitlab/-/raw/master/doc/api/projects.md")
	if err != nil {
		return nil, errors.Wrap(err, `failed to get GitLab project configuration descriptions`)
	}
	return parseProjectTable(data)
}

func saveConfiguration(configuration *Configuration, descriptions map[string]string, output string) errors.E {
	node := &yaml.Node{}
	err := node.Encode(configuration)
	if err != nil {
		return errors.Wrap(err, `cannot encode configuration`)
	}
	return writeYAML(node, descriptions, output)
}

func writeYAML(node *yaml.Node, descriptions map[string]string, output string) errors.E {
	if node.Kind != yaml.MappingNode {
		return errors.Errorf(`invalid node kind: %d`, node.Kind)
	}
	if node.Content[0].Value != "project" {
		return errors.Errorf(`invalid node structure, missing "project"`)
	}
	for i := 0; i < len(node.Content[1].Content); i += 2 {
		description, ok := descriptions[node.Content[1].Content[i].Value]
		if ok && node.Content[1].Content[i].HeadComment == "" {
			node.Content[1].Content[i].HeadComment = wordwrap.WrapString(description, 80)
		}
	}

	buffer := bytes.Buffer{}

	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(2)
	err := encoder.Encode(node)
	if err != nil {
		return errors.Wrap(err, `cannot marshal configuration`)
	}
	err = encoder.Close()
	if err != nil {
		return errors.Wrap(err, `cannot marshal configuration`)
	}

	if output != "-" {
		err = os.WriteFile(kong.ExpandPath(output), buffer.Bytes(), 0o644)
	} else {
		_, err = os.Stdout.Write(buffer.Bytes())
	}
	if err != nil {
		return errors.Wrapf(err, `cannot write configuration to "%s"`, output)
	}

	return nil
}

func (c *SaveCommand) Run(globals *Globals) errors.E {
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

	descriptions, errE := getProjectConfigDescriptions()
	if errE != nil {
		return errE
	}

	client, err := gitlab.NewClient(c.Token, gitlab.WithBaseURL(c.BaseURL))
	if err != nil {
		return errors.Wrap(err, `failed to create GitLab API client instance`)
	}

	configuration := Configuration{}

	errE = getProjectConfig(client, c.Project, descriptions, &configuration)
	if errE != nil {
		return errE
	}

	return saveConfiguration(&configuration, descriptions, c.Output)
}
