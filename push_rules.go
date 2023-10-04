package config

import (
	"fmt"
	"net/http"
	"os"

	"github.com/xanzy/go-gitlab"
	"gitlab.com/tozd/go/errors"
)

func getPushRules(client *gitlab.Client, project string) (map[string]interface{}, errors.E) {
	u := fmt.Sprintf("projects/%s/push_rule", gitlab.PathEscape(project))
	req, err := client.NewRequest(http.MethodGet, u, nil, nil)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to get push rules")
	}

	pushRules := map[string]interface{}{}

	_, err = client.Do(req, &pushRules)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to get push rules")
	}

	return pushRules, nil
}

// getPushRules populates configuration struct with GitLab's project's
// push rules available from GitLab push rules API endpoint.
func (c *GetCommand) getPushRules(client *gitlab.Client, configuration *Configuration) (bool, errors.E) { //nolint:unparam
	fmt.Fprintf(os.Stderr, "Getting push rules...\n")

	configuration.PushRules = map[string]interface{}{}

	descriptions, errE := getPushRulesDescriptions(c.DocsRef)
	if errE != nil {
		return false, errE
	}

	pushRules, errE := getPushRules(client, c.Project)
	if errE != nil {
		return false, errE
	}

	// Only retain those keys which can be edited through the API
	// (which are those available in descriptions).
	for key := range pushRules {
		_, ok := descriptions[key]
		if !ok {
			delete(pushRules, key)
		}
	}

	// Add comments for keys. We process these keys before writing YAML out.
	describeKeys(pushRules, descriptions)

	configuration.PushRules = pushRules

	return false, nil
}

// parsePushRulesDocumentation parses GitLab's documentation in Markdown for
// push rules API endpoint and extracts description of fields used to describe
// payload for project's push rules.
func parsePushRulesDocumentation(input []byte) (map[string]string, errors.E) {
	return parseTable(input, "Edit project push rule", nil)
}

// getPushRulesDescriptions obtains description of fields used to describe payload for
// project's push rules from GitLab's documentation for push rules API endpoint.
func getPushRulesDescriptions(gitRef string) (map[string]string, errors.E) {
	data, err := downloadFile(fmt.Sprintf("https://gitlab.com/gitlab-org/gitlab/-/raw/%s/doc/api/projects.md", gitRef))
	if err != nil {
		return nil, errors.WithMessage(err, "failed to get push rules descriptions")
	}
	return parsePushRulesDocumentation(data)
}

// updatePushRules updates GitLab project's push rules
// using GitLab push rules API endpoint based on the configuration struct.
func (c *SetCommand) updatePushRules(client *gitlab.Client, configuration *Configuration) errors.E {
	if configuration.PushRules == nil {
		return nil
	}

	fmt.Fprintf(os.Stderr, "Updating push rules...\n")

	pushRules, errE := getPushRules(client, c.Project)
	if errE != nil {
		return errE
	}

	if len(configuration.PushRules) == 0 {
		// The call is not really idempotent, so we delete rules only if they exist.
		// See: https://gitlab.com/gitlab-org/gitlab/-/issues/427352
		if len(pushRules) > 0 {
			_, err := client.Projects.DeleteProjectPushRule(c.Project)
			if err != nil {
				return errors.WithMessage(err, "failed to delete push rules")
			}
		}
		return nil
	}

	var method string
	var description string
	if len(pushRules) == 0 {
		method = http.MethodPost
		description = "create"
	} else {
		method = http.MethodPut
		description = "update"
	}

	u := fmt.Sprintf("projects/%s/push_rule", gitlab.PathEscape(c.Project))
	req, err := client.NewRequest(method, u, configuration.PushRules, nil)
	if err != nil {
		return errors.WithMessagef(err, "failed to %s push rules", description)
	}
	_, err = client.Do(req, nil)
	if err != nil {
		return errors.WithMessagef(err, "failed to %s push rules", description)
	}

	return nil
}
