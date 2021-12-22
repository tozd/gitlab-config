package config

import (
	"fmt"
	"net/http"
	"sort"

	mapset "github.com/deckarep/golang-set"
	"github.com/google/go-querystring/query"
	"github.com/xanzy/go-gitlab"
	"gitlab.com/tozd/go/errors"
)

type filter struct {
	EnvironmentScope string `url:"environment_scope"`
}

type opts struct {
	Filter filter `url:"filter"`
}

// getVariables populates configuration struct with configuration available
// from GitLab project level variables API endpoint.
func (c *GetCommand) getVariables(client *gitlab.Client, configuration *Configuration) (bool, errors.E) {
	fmt.Printf("Getting variables...\n")

	configuration.Variables = []map[string]interface{}{}

	descriptions, errE := getVariablesDescriptions(c.DocsRef)
	if errE != nil {
		return false, errE
	}
	configuration.VariablesComment = formatDescriptions(descriptions)

	u := fmt.Sprintf("projects/%s/variables", gitlab.PathEscape(c.Project))
	options := &gitlab.ListProjectVariablesOptions{
		PerPage: maxGitLabPageSize,
		Page:    1,
	}

	for {
		req, err := client.NewRequest(http.MethodGet, u, options, nil)
		if err != nil {
			return false, errors.Wrapf(err, `failed to get variables, page %d`, options.Page)
		}

		variables := []map[string]interface{}{}

		response, err := client.Do(req, &variables)
		if err != nil {
			return false, errors.Wrapf(err, `failed to get variables, page %d`, options.Page)
		}

		if len(variables) == 0 {
			break
		}

		for _, variable := range variables {
			// Only retain those keys which can be edited through the API
			// (which are those available in descriptions).
			for key := range variable {
				_, ok := descriptions[key]
				if !ok {
					delete(variable, key)
				}
			}

			if c.EncComment != "" {
				variable["comment:value"+c.EncSuffix] = c.EncComment
			}
			if c.EncSuffix != "" {
				variable["value"+c.EncSuffix] = variable["value"]
				delete(variable, "value")
			}

			configuration.Variables = append(configuration.Variables, variable)
		}

		if response.NextPage == 0 {
			break
		}

		options.Page = response.NextPage
	}

	// We sort by variable key so that we have deterministic order.
	sort.Slice(configuration.Variables, func(i, j int) bool {
		return configuration.Variables[i]["key"].(string) < configuration.Variables[j]["key"].(string)
	})

	return len(configuration.Variables) > 0, nil
}

// parseVariablesDocumentation parses GitLab's documentation in Markdown for
// project level variables API endpoint and extracts description of fields
// used to describe an individual variable.
func parseVariablesDocumentation(input []byte) (map[string]string, errors.E) {
	return parseTable(input, "Create variable", nil)
}

// getVariablesDescriptions obtains description of fields used to describe an individual
// variable from GitLab's documentation for project level variables API endpoint.
func getVariablesDescriptions(gitRef string) (map[string]string, errors.E) {
	data, err := downloadFile(fmt.Sprintf("https://gitlab.com/gitlab-org/gitlab/-/raw/%s/doc/api/project_level_variables.md", gitRef)) //nolint:lll
	if err != nil {
		return nil, errors.Wrap(err, `failed to get variables descriptions`)
	}
	return parseVariablesDocumentation(data)
}

// updateVariables updates GitLab project's variables using GitLab project level
// variables API endpoint based on the configuration struct.
func (c *SetCommand) updateVariables(client *gitlab.Client, configuration *Configuration) errors.E {
	if configuration.Variables == nil {
		return nil
	}

	fmt.Printf("Updating variables...\n")

	options := &gitlab.ListProjectVariablesOptions{
		PerPage: maxGitLabPageSize,
		Page:    1,
	}

	variables := []*gitlab.ProjectVariable{}

	for {
		vs, response, err := client.ProjectVariables.ListVariables(c.Project, options)
		if err != nil {
			return errors.Wrapf(err, `failed to get variables, page %d`, options.Page)
		}

		variables = append(variables, vs...)

		if response.NextPage == 0 {
			break
		}

		options.Page = response.NextPage
	}

	type Variable struct {
		Key              string
		EnvironmentScope string
	}

	existingVariables := mapset.NewThreadUnsafeSet()
	for _, variable := range variables {
		existingVariables.Add(Variable{
			Key:              variable.Key,
			EnvironmentScope: variable.EnvironmentScope,
		})
	}
	wantedVariables := mapset.NewThreadUnsafeSet()
	for i, variable := range configuration.Variables {
		key, ok := variable["key"]
		if !ok {
			return errors.Errorf(`variable in configuration at index %d does not have "key"`, i)
		}
		k, ok := key.(string)
		if !ok {
			return errors.Errorf(`invalid "key" in "variables" at index %d`, i)
		}
		environmentScope, ok := variable["environment_scope"]
		if !ok {
			return errors.Errorf(`variable in configuration at index %d does not have "environment_scope"`, i)
		}
		e, ok := environmentScope.(string)
		if !ok {
			return errors.Errorf(`invalid "environment_scope" in "variables" at index %d`, i)
		}
		wantedVariables.Add(Variable{
			Key:              k,
			EnvironmentScope: e,
		})
	}

	extraVariables := existingVariables.Difference(wantedVariables)
	for _, extraVariable := range extraVariables.ToSlice() {
		variable := extraVariable.(Variable) //nolint:errcheck
		// TODO: Use go-gitlab's function once it is updated to new API.
		//       See: https://github.com/xanzy/go-gitlab/issues/1328
		u := fmt.Sprintf("projects/%s/variables/%s", gitlab.PathEscape(c.Project), gitlab.PathEscape(variable.Key))
		req, err := client.NewRequest(http.MethodDelete, u, opts{filter{variable.EnvironmentScope}}, nil)
		if err != nil {
			return errors.Wrapf(err, `failed to remove variable "%s"/"%s"`, variable.Key, variable.EnvironmentScope)
		}
		_, err = client.Do(req, nil)
		if err != nil {
			return errors.Wrapf(err, `failed to remove variable "%s"/"%s"`, variable.Key, variable.EnvironmentScope)
		}
	}

	for _, variable := range configuration.Variables {
		// We made sure above that all variables in configuration have a string key and environment scope.
		key := variable["key"].(string)                            //nolint:errcheck
		environmentScope := variable["environment_scope"].(string) //nolint:errcheck

		if existingVariables.Contains(Variable{
			Key:              key,
			EnvironmentScope: environmentScope,
		}) {
			// Update existing variable.
			u := fmt.Sprintf("projects/%s/variables/%s", gitlab.PathEscape(c.Project), gitlab.PathEscape(key))
			req, err := client.NewRequest(http.MethodPut, u, variable, nil)
			if err != nil {
				return errors.Wrapf(err, `failed to update variable "%s"/"%s"`, key, environmentScope)
			}
			q, err := query.Values(opts{filter{environmentScope}})
			if err != nil {
				return errors.Wrapf(err, `failed to update variable "%s"/"%s"`, key, environmentScope)
			}
			req.URL.RawQuery = q.Encode()
			_, err = client.Do(req, nil)
			if err != nil {
				return errors.Wrapf(err, `failed to update variable "%s"/"%s"`, key, environmentScope)
			}
		} else {
			// Create new variable.
			u := fmt.Sprintf("projects/%s/variables", gitlab.PathEscape(c.Project))
			req, err := client.NewRequest(http.MethodPost, u, variable, nil)
			if err != nil {
				return errors.Wrapf(err, `failed to create variable "%s"/"%s"`, key, environmentScope)
			}
			_, err = client.Do(req, nil)
			if err != nil {
				return errors.Wrapf(err, `failed to create variable "%s"/"%s"`, key, environmentScope)
			}
		}
	}

	return nil
}
