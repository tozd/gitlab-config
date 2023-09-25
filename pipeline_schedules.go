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

			// Only retain those keys which can be edited through the API
			// (which are those available in descriptions).
			for key := range pipelineSchedule {
				_, ok := descriptions[key]
				if !ok {
					delete(pipelineSchedule, key)
				}
			}

			id, ok := pipelineSchedule["id"]
			if !ok {
				return false, errors.New(`pipeline schedule is missing field "id"`)
			}
			_, ok = id.(int)
			if !ok {
				errE := errors.New(`pipeline schedule's field "id" is not an integer`)
				errors.Details(errE)["type"] = fmt.Sprintf("%T", id)
				errors.Details(errE)["value"] = id
				return false, errE
			}

			configuration.PipelineSchedules = append(configuration.PipelineSchedules, pipelineSchedule)
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
func (c *SetCommand) updatePipelineSchedules(client *gitlab.Client, configuration *Configuration) errors.E {
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
		pt, response, err := client.PipelineSchedules.ListPipelineSchedules(c.Project, options)
		if err != nil {
			errE := errors.WithMessage(err, "failed to get pipeline schedules")
			errors.Details(errE)["page"] = options.Page
			return errE
		}

		pipelineSchedules = append(pipelineSchedules, pt...)

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
		if !ok {
			errE := errors.Errorf(`pipeline schedule is missing field "id"`)
			errors.Details(errE)["index"] = i
			return errE
		}
		iid, ok := id.(int)
		if !ok {
			errE := errors.New(`pipeline schedule's field "id" is not an integer`)
			errors.Details(errE)["index"] = i
			errors.Details(errE)["type"] = fmt.Sprintf("%T", id)
			errors.Details(errE)["value"] = id
			return errE
		}
		wantedPipelineSchedulesSet.Add(iid)
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

	for _, pipelineSchedule := range configuration.PipelineSchedules {
		// We made sure above that all pipeline schedules in configuration have an integer ID.
		id := pipelineSchedule["id"].(int) //nolint:errcheck,forcetypeassert

		if existingPipelineSchedulesSet.Contains(id) {
			// Update existing pipeline schedule.
			u := fmt.Sprintf("projects/%s/pipeline_schedules/%d", gitlab.PathEscape(c.Project), id)
			req, err := client.NewRequest(http.MethodPut, u, pipelineSchedule, nil)
			if err != nil {
				errE := errors.WithMessage(err, "failed to update pipeline schedule")
				errors.Details(errE)["pipelineSchedule"] = id
			}
			_, err = client.Do(req, nil)
			if err != nil {
				errE := errors.WithMessage(err, "failed to update pipeline schedule")
				errors.Details(errE)["pipelineSchedule"] = id
			}
		} else {
			// Create new pipeline schedule.
			u := fmt.Sprintf("projects/%s/pipeline_schedules", gitlab.PathEscape(c.Project))
			req, err := client.NewRequest(http.MethodPost, u, pipelineSchedule, nil)
			if err != nil {
				errE := errors.WithMessage(err, "failed to create pipeline schedule")
				errors.Details(errE)["pipelineSchedule"] = id
			}
			_, err = client.Do(req, nil)
			if err != nil {
				errE := errors.WithMessage(err, "failed to create pipeline schedule")
				errors.Details(errE)["pipelineSchedule"] = id
				return errE
			}
		}
	}

	return nil
}
