package config

import (
	"fmt"
	"net/http"
	"os"
	"sort"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/xanzy/go-gitlab"
	"gitlab.com/tozd/go/errors"
)

// getPipelineSchedules populates configuration struct with configuration available
// from GitLab pipeline schedules API endpoint.
func (c *GetCommand) getPipelineSchedules(client *gitlab.Client, configuration *Configuration) (bool, errors.E) { //nolint:unparam
	fmt.Fprintf(os.Stderr, "Getting pipeline schedules...\n")

	configuration.PipelineSchedules = []map[string]interface{}{}

	descriptions, errE := getPipelineSchedulesDescriptions(c.DocsRef)
	if errE != nil {
		return false, errE
	}
	// We need "id" later on.
	if _, ok := descriptions["id"]; !ok {
		return false, errors.New(`"id" field is missing in pipeline schedules descriptions`)
	}
	configuration.PipelineSchedulesComment = formatDescriptions(descriptions)

	u := fmt.Sprintf("projects/%s/pipeline_schedules", gitlab.PathEscape(c.Project))
	options := &gitlab.ListPipelineSchedulesOptions{
		PerPage: maxGitLabPageSize,
		Page:    1,
	}

	for {
		req, err := client.NewRequest(http.MethodGet, u, options, nil)
		if err != nil {
			errE := errors.WithMessage(err, "failed to get pipeline schedules")
			errors.Details(errE)["page"] = options.Page
			return false, errE
		}

		pipelineSchedules := []map[string]interface{}{}

		response, err := client.Do(req, &pipelineSchedules)
		if err != nil {
			errE := errors.WithMessage(err, "failed to get pipeline schedules")
			errors.Details(errE)["page"] = options.Page
			return false, errE
		}

		if len(pipelineSchedules) == 0 {
			break
		}

		for _, pipelineSchedule := range pipelineSchedules {
			// Making sure ids are an integer.
			castFloatsToInts(pipelineSchedule)

			id, ok := pipelineSchedule["id"]
			if !ok {
				return false, errors.New(`pipeline schedule is missing field "id"`)
			}
			iid, ok := id.(int)
			if !ok {
				errE := errors.New(`pipeline schedule's field "id" is not an integer`)
				errors.Details(errE)["type"] = fmt.Sprintf("%T", id)
				errors.Details(errE)["value"] = id
				return false, errE
			}

			// We have to fetch each pipeline schedule individually to get variables.
			req, err := client.NewRequest(http.MethodGet, fmt.Sprintf("projects/%s/pipeline_schedules/%d", gitlab.PathEscape(c.Project), iid), nil, nil)
			if err != nil {
				errE := errors.WithMessage(err, "failed to get pipeline schedule")
				errors.Details(errE)["id"] = iid
				return false, errE
			}

			ps := map[string]interface{}{}

			_, err = client.Do(req, &ps)
			if err != nil {
				errE := errors.WithMessage(err, "failed to get pipeline schedule")
				errors.Details(errE)["id"] = iid
				return false, errE
			}

			// Making sure ids are an integer.
			castFloatsToInts(ps)

			// We already extracted ID, so we just set it to not have to validate it again.
			ps["id"] = iid

			// Only retain those keys which can be edited through the API
			// (which are those available in descriptions).
			for key := range ps {
				_, ok := descriptions[key]
				if !ok {
					delete(ps, key)
				}
			}

			// TODO: This field is returned, but it cannot be changed. It is not documented.
			//       See: https://gitlab.com/gitlab-org/gitlab/-/issues/427328
			removeField(ps, "raw")

			configuration.PipelineSchedules = append(configuration.PipelineSchedules, ps)
		}

		if response.NextPage == 0 {
			break
		}

		options.Page = response.NextPage
	}

	// We sort by pipeline schedule's id so that we have deterministic order.
	sort.Slice(configuration.PipelineSchedules, func(i, j int) bool {
		// We checked that id is int above.
		return configuration.PipelineSchedules[i]["id"].(int) < configuration.PipelineSchedules[j]["id"].(int) //nolint:forcetypeassert
	})

	// For now pipeline schedule variables cannot contain secrets as they cannot be masked,
	// so we return false here.
	// See: https://gitlab.com/gitlab-org/gitlab/-/issues/35439
	return false, nil
}

// parsePipelineSchedulesDocumentation parses GitLab's documentation in Markdown for
// pipeline schedules API endpoint and extracts description of fields used to describe
// pipeline schedules.
func parsePipelineSchedulesDocumentation(input []byte) (map[string]string, errors.E) {
	descriptions, err := parseTable(input, "Edit a pipeline schedule", nil)
	if err != nil {
		return nil, err
	}
	descriptions["id"] = descriptions["pipeline_schedule_id"]
	delete(descriptions, "pipeline_schedule_id")
	descriptions["variables"] = `Array of variables, with each described by a hash of the form {key: string, value: string, variable_type: string}. Type: array`
	return descriptions, nil
}

// getPipelineSchedulesDescriptions obtains description of fields used to describe
// an individual pipeline schedules from GitLab's documentation for pipeline schedules API endpoint.
func getPipelineSchedulesDescriptions(gitRef string) (map[string]string, errors.E) {
	data, err := downloadFile(fmt.Sprintf("https://gitlab.com/gitlab-org/gitlab/-/raw/%s/doc/api/pipeline_schedules.md", gitRef))
	if err != nil {
		return nil, errors.WithMessage(err, "failed to get pipeline schedules descriptions")
	}
	return parsePipelineSchedulesDocumentation(data)
}

// updatePipelineSchedules updates GitLab project's pipeline schedules using GitLab
// pipeline schedules API endpoint based on the configuration struct.
func (c *SetCommand) updatePipelineSchedules(client *gitlab.Client, configuration *Configuration) errors.E { //nolint:maintidx
	if configuration.PipelineSchedules == nil {
		return nil
	}

	fmt.Fprintf(os.Stderr, "Updating pipeline schedules...\n")

	options := &gitlab.ListPipelineSchedulesOptions{
		PerPage: maxGitLabPageSize,
		Page:    1,
	}

	pipelineSchedules := []*gitlab.PipelineSchedule{}

	for {
		ps, response, err := client.PipelineSchedules.ListPipelineSchedules(c.Project, options)
		if err != nil {
			errE := errors.WithMessage(err, "failed to get pipeline schedules")
			errors.Details(errE)["page"] = options.Page
			return errE
		}

		pipelineSchedules = append(pipelineSchedules, ps...)

		if response.NextPage == 0 {
			break
		}

		options.Page = response.NextPage
	}

	existingPipelineSchedulesSet := mapset.NewThreadUnsafeSet[int]()
	for _, pipelineSchedule := range pipelineSchedules {
		existingPipelineSchedulesSet.Add(pipelineSchedule.ID)
	}

	wantedPipelineSchedulesSet := mapset.NewThreadUnsafeSet[int]()
	for i, pipelineSchedule := range configuration.PipelineSchedules {
		id, ok := pipelineSchedule["id"]
		if ok {
			// If ID is provided, the pipeline schedule should exist.
			iid, ok := id.(int)
			if !ok {
				errE := errors.New(`pipeline schedule's field "id" is not an integer`)
				errors.Details(errE)["index"] = i
				errors.Details(errE)["type"] = fmt.Sprintf("%T", id)
				errors.Details(errE)["value"] = id
				return errE
			}
			wantedPipelineSchedulesSet.Add(iid)
			if existingPipelineSchedulesSet.Contains(iid) {
				continue
			}
			// Pipeline schedule does not exist with that ID.
			// We remove the ID and create a new pipeline schedule.
			delete(pipelineSchedule, "id")
		}
	}

	extraPipelineSchedulesSet := existingPipelineSchedulesSet.Difference(wantedPipelineSchedulesSet)
	for _, pipelineScheduleID := range extraPipelineSchedulesSet.ToSlice() {
		_, err := client.PipelineSchedules.DeletePipelineSchedule(c.Project, pipelineScheduleID)
		if err != nil {
			errE := errors.WithMessage(err, "failed to delete pipeline schedule")
			errors.Details(errE)["pipelineSchedule"] = pipelineScheduleID
			return errE
		}
	}

	for i, pipelineSchedule := range configuration.PipelineSchedules {
		var ps *gitlab.PipelineSchedule

		id, ok := pipelineSchedule["id"]
		if !ok {
			u := fmt.Sprintf("projects/%s/pipeline_schedules", gitlab.PathEscape(c.Project))
			req, err := client.NewRequest(http.MethodPost, u, pipelineSchedule, nil)
			if err != nil {
				errE := errors.WithMessage(err, "failed to create pipeline schedule")
				errors.Details(errE)["index"] = i
			}
			ps = new(gitlab.PipelineSchedule)
			_, err = client.Do(req, ps)
			if err != nil {
				errE := errors.WithMessage(err, "failed to create pipeline schedule")
				errors.Details(errE)["index"] = i
				return errE
			}
		} else {
			// We made sure above that all pipeline schedules in configuration with pipeline schedule
			// ID exist and that they are ints.
			iid := id.(int) //nolint:errcheck,forcetypeassert
			u := fmt.Sprintf("projects/%s/pipeline_schedules/%d", gitlab.PathEscape(c.Project), iid)
			req, err := client.NewRequest(http.MethodPut, u, pipelineSchedule, nil)
			if err != nil {
				errE := errors.WithMessage(err, "failed to update pipeline schedule")
				errors.Details(errE)["index"] = i
				errors.Details(errE)["pipelineSchedule"] = iid
			}
			_, err = client.Do(req, nil)
			if err != nil {
				errE := errors.WithMessage(err, "failed to update pipeline schedule")
				errors.Details(errE)["index"] = i
				errors.Details(errE)["pipelineSchedule"] = iid
				return errE
			}

			ps, _, err = client.PipelineSchedules.GetPipelineSchedule(c.Project, iid)
			if err != nil {
				errE := errors.WithMessage(err, "failed to get pipeline schedule")
				errors.Details(errE)["index"] = i
				errors.Details(errE)["pipelineSchedule"] = iid
				return errE
			}
		}

		existingVariablesSet := mapset.NewThreadUnsafeSet[string]()
		for _, variable := range ps.Variables {
			existingVariablesSet.Add(variable.Key)
		}

		wantedVariables, ok := pipelineSchedule["variables"]
		if !ok {
			wantedVariables = []interface{}{}
		}

		variables, ok := wantedVariables.([]interface{})
		if !ok {
			errE := errors.New("invalid variables for pipeline schedule")
			errors.Details(errE)["index"] = i
			errors.Details(errE)["pipelineSchedule"] = ps.ID
			return errE
		}

		wantedVariablesSet := mapset.NewThreadUnsafeSet[string]()
		for j, variable := range variables {
			v, ok := variable.(map[string]interface{})
			if !ok {
				errE := errors.New("invalid variable for pipeline schedule")
				errors.Details(errE)["index"] = i
				errors.Details(errE)["variableIndex"] = j
				errors.Details(errE)["pipelineSchedule"] = ps.ID
				return errE
			}
			key, ok := v["key"]
			if !ok {
				errE := errors.Errorf(`variable for pipeline schedule is missing field "key"`)
				errors.Details(errE)["index"] = i
				errors.Details(errE)["variableIndex"] = j
				errors.Details(errE)["pipelineSchedule"] = ps.ID
				return errE
			}
			k, ok := key.(string)
			if !ok {
				errE := errors.New(`variable's field "key" for pipeline schedule is not a string`)
				errors.Details(errE)["index"] = i
				errors.Details(errE)["variableIndex"] = j
				errors.Details(errE)["pipelineSchedule"] = ps.ID
				errors.Details(errE)["type"] = fmt.Sprintf("%T", key)
				errors.Details(errE)["value"] = key
				return errE
			}
			wantedVariablesSet.Add(k)
		}

		extraVariablesSet := existingVariablesSet.Difference(wantedVariablesSet)
		for _, variable := range extraVariablesSet.ToSlice() {
			_, _, err := client.PipelineSchedules.DeletePipelineScheduleVariable(
				c.Project,
				ps.ID,
				variable,
			)
			if err != nil {
				errE := errors.WithMessage(err, "failed to remove variable for pipeline schedule")
				errors.Details(errE)["index"] = i
				errors.Details(errE)["pipelineSchedule"] = ps.ID
				errors.Details(errE)["key"] = variable
				return errE
			}
		}

		for j, variable := range variables {
			// We made sure above that all variables in configuration have a string key.
			v := variable.(map[string]interface{}) //nolint:errcheck,forcetypeassert
			key := v["key"].(string)               //nolint:errcheck,forcetypeassert

			if existingVariablesSet.Contains(key) {
				// Update existing variable.
				u := fmt.Sprintf("projects/%s/pipeline_schedules/%d/variables/%s", gitlab.PathEscape(c.Project), ps.ID, gitlab.PathEscape(key))
				req, err := client.NewRequest(http.MethodPut, u, variable, nil)
				if err != nil {
					errE := errors.WithMessage(err, "failed to update variable for pipeline schedule")
					errors.Details(errE)["index"] = i
					errors.Details(errE)["variableIndex"] = j
					errors.Details(errE)["pipelineSchedule"] = ps.ID
					errors.Details(errE)["key"] = key
				}
				_, err = client.Do(req, nil)
				if err != nil {
					errE := errors.WithMessage(err, "failed to update variable for pipeline schedule")
					errors.Details(errE)["index"] = i
					errors.Details(errE)["variableIndex"] = j
					errors.Details(errE)["pipelineSchedule"] = ps.ID
					errors.Details(errE)["key"] = key
					return errE
				}
			} else {
				// Create new variable.
				u := fmt.Sprintf("projects/%s/pipeline_schedules/%d/variables", gitlab.PathEscape(c.Project), ps.ID)
				req, err := client.NewRequest(http.MethodPost, u, variable, nil)
				if err != nil {
					errE := errors.WithMessage(err, "failed to create variable for pipeline schedule")
					errors.Details(errE)["index"] = i
					errors.Details(errE)["variableIndex"] = j
					errors.Details(errE)["pipelineSchedule"] = ps.ID
					errors.Details(errE)["key"] = key
				}
				_, err = client.Do(req, nil)
				if err != nil {
					errE := errors.WithMessage(err, "failed to create variable for pipeline schedule")
					errors.Details(errE)["index"] = i
					errors.Details(errE)["variableIndex"] = j
					errors.Details(errE)["pipelineSchedule"] = ps.ID
					errors.Details(errE)["key"] = key
					return errE
				}
			}
		}
	}

	return nil
}
