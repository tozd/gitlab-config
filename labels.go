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

// getLabels populates configuration struct with configuration available
// from GitLab labels API endpoint.
func (c *GetCommand) getLabels(client *gitlab.Client, configuration *Configuration) (bool, errors.E) { //nolint:unparam
	fmt.Fprintf(os.Stderr, "Getting labels...\n")

	configuration.Labels = []map[string]interface{}{}

	descriptions, errE := getLabelsDescriptions(c.DocsRef)
	if errE != nil {
		return false, errE
	}
	// We need "id" later on.
	if _, ok := descriptions["id"]; !ok {
		return false, errors.New(`"id" field is missing in labels descriptions`)
	}
	configuration.LabelsComment = formatDescriptions(descriptions)

	u := fmt.Sprintf("projects/%s/labels", gitlab.PathEscape(c.Project))
	options := &gitlab.ListLabelsOptions{ //nolint:exhaustruct
		ListOptions: gitlab.ListOptions{
			PerPage: maxGitLabPageSize,
			Page:    1,
		},
		IncludeAncestorGroups: gitlab.Bool(false),
	}

	for { //nolint:dupl
		req, err := client.NewRequest(http.MethodGet, u, options, nil)
		if err != nil {
			errE := errors.WithMessage(err, "failed to get project labels")
			errors.Details(errE)["page"] = options.Page
			return false, errE
		}

		labels := []map[string]interface{}{}

		response, err := client.Do(req, &labels)
		if err != nil {
			errE := errors.WithMessage(err, "failed to get project labels")
			errors.Details(errE)["page"] = options.Page
			return false, errE
		}

		if len(labels) == 0 {
			break
		}

		for _, label := range labels {
			// Making sure id and priority are an integer.
			castFloatsToInts(label)

			// Only retain those keys which can be edited through the API
			// (which are those available in descriptions).
			for key := range label {
				_, ok := descriptions[key]
				if !ok {
					delete(label, key)
				}
			}

			id, ok := label["id"]
			if !ok {
				return false, errors.New(`label is missing field "id"`)
			}
			_, ok = id.(int)
			if !ok {
				errE := errors.New(`label's field "id" is not an integer`)
				errors.Details(errE)["type"] = fmt.Sprintf("%T", id)
				errors.Details(errE)["value"] = id
				return false, errE
			}

			configuration.Labels = append(configuration.Labels, label)
		}

		if response.NextPage == 0 {
			break
		}

		options.Page = response.NextPage
	}

	// We sort by label ID so that we have deterministic order.
	sort.Slice(configuration.Labels, func(i, j int) bool {
		// We checked that id is int above.
		return configuration.Labels[i]["id"].(int) < configuration.Labels[j]["id"].(int) //nolint:forcetypeassert
	})

	return false, nil
}

// parseLabelsDocumentation parses GitLab's documentation in Markdown for
// labels API endpoint and extracts description of fields used to describe
// an individual label.
func parseLabelsDocumentation(input []byte) (map[string]string, errors.E) {
	newDescriptions, err := parseTable(input, "Create a new label", nil)
	if err != nil {
		return nil, err
	}
	editDescriptions, err := parseTable(input, "Edit an existing label", nil)
	if err != nil {
		return nil, err
	}
	// We want to preserve label IDs so we copy edit description for it.
	newDescriptions["id"] = editDescriptions["label_id"]
	return newDescriptions, nil
}

// getLabelsDescriptions obtains description of fields used to describe
// an individual label from GitLab's documentation for labels API endpoint.
func getLabelsDescriptions(gitRef string) (map[string]string, errors.E) {
	data, err := downloadFile(fmt.Sprintf("https://gitlab.com/gitlab-org/gitlab/-/raw/%s/doc/api/labels.md", gitRef))
	if err != nil {
		return nil, errors.WithMessage(err, "failed to get project labels descriptions")
	}
	return parseLabelsDocumentation(data)
}

// updateLabels updates GitLab project's labels using GitLab labels API endpoint
// based on the configuration struct.
//
// Labels without the ID field are matched to existing labels based on the name.
// Unmatched labels are created as new. Save configuration with label IDs to be able
// to rename existing labels.
func (c *SetCommand) updateLabels(client *gitlab.Client, configuration *Configuration) errors.E {
	if configuration.Labels == nil {
		return nil
	}

	fmt.Fprintf(os.Stderr, "Updating labels...\n")

	options := &gitlab.ListLabelsOptions{ //nolint:exhaustruct
		ListOptions: gitlab.ListOptions{
			PerPage: maxGitLabPageSize,
			Page:    1,
		},
		IncludeAncestorGroups: gitlab.Bool(false),
	}

	labels := []*gitlab.Label{}

	for {
		ls, response, err := client.Labels.ListLabels(c.Project, options)
		if err != nil {
			errE := errors.WithMessage(err, "failed to get project labels")
			errors.Details(errE)["page"] = options.Page
			return errE
		}

		labels = append(labels, ls...)

		if response.NextPage == 0 {
			break
		}

		options.Page = response.NextPage
	}

	existingLabelsSet := mapset.NewThreadUnsafeSet[int]()
	namesToIDs := map[string]int{}
	for _, label := range labels {
		namesToIDs[label.Name] = label.ID
		existingLabelsSet.Add(label.ID)
	}

	// Set label IDs if a matching existing label can be found.
	for i, label := range configuration.Labels {
		// Is label ID already set?
		id, ok := label["id"]
		if ok {
			// If ID is provided, the label should exist.
			iid, ok := id.(int) //nolint:govet
			if !ok {
				errE := errors.New(`project label's field "id" is not an integer`)
				errors.Details(errE)["index"] = i
				errors.Details(errE)["type"] = fmt.Sprintf("%T", id)
				errors.Details(errE)["value"] = id
				return errE
			}
			if existingLabelsSet.Contains(iid) {
				continue
			}
			// Label does not exist with that ID. We remove the ID and leave to matching to
			// find the correct ID, if it exists. Otherwise we will just create a new label.
			delete(label, "id")
		}

		name, ok := label["name"]
		if !ok {
			errE := errors.Errorf(`project label is missing field "name"`)
			errors.Details(errE)["index"] = i
			return errE
		}
		n, ok := name.(string)
		if ok {
			id, ok = namesToIDs[n]
			if ok {
				label["id"] = id
			}
		}
	}

	wantedLabelsSet := mapset.NewThreadUnsafeSet[int]()
	for _, label := range configuration.Labels {
		id, ok := label["id"]
		if ok {
			// We checked that id is int above.
			wantedLabelsSet.Add(id.(int)) //nolint:forcetypeassert
		}
	}

	extraLabelsSet := existingLabelsSet.Difference(wantedLabelsSet)
	for _, labelID := range extraLabelsSet.ToSlice() {
		// TODO: Use go-gitlab's function once it is updated to new API.
		//       See: https://github.com/xanzy/go-gitlab/issues/1321
		u := fmt.Sprintf("projects/%s/labels/%d", gitlab.PathEscape(c.Project), labelID)
		req, err := client.NewRequest(http.MethodDelete, u, nil, nil)
		if err != nil {
			errE := errors.WithMessage(err, "failed to delete project label")
			errors.Details(errE)["label"] = labelID
			return errE
		}
		_, err = client.Do(req, nil)
		if err != nil {
			errE := errors.WithMessage(err, "failed to delete project label")
			errors.Details(errE)["label"] = labelID
			return errE
		}
	}

	for _, label := range configuration.Labels {
		id, ok := label["id"]
		if !ok { //nolint:dupl
			u := fmt.Sprintf("projects/%s/labels", gitlab.PathEscape(c.Project))
			req, err := client.NewRequest(http.MethodPost, u, label, nil)
			if err != nil {
				// We made sure above that all labels in configuration without label ID have name.
				errE := errors.WithMessage(err, "failed to create project label")
				errors.Details(errE)["label"] = label["name"]
				return errE
			}
			_, err = client.Do(req, nil)
			if err != nil { // We made sure above that all labels in configuration without label ID have name.
				errE := errors.WithMessage(err, "failed to create project label")
				errors.Details(errE)["label"] = label["name"]
				return errE
			}
		} else {
			// We made sure above that all labels in configuration with label ID exist
			// and that they are ints.
			iid := id.(int) //nolint:errcheck,forcetypeassert
			u := fmt.Sprintf("projects/%s/labels/%d", gitlab.PathEscape(c.Project), iid)
			req, err := client.NewRequest(http.MethodPut, u, label, nil)
			if err != nil {
				errE := errors.WithMessage(err, "failed to update project label")
				errors.Details(errE)["label"] = iid
				return errE
			}
			_, err = client.Do(req, nil)
			if err != nil {
				errE := errors.WithMessage(err, "failed to update project label")
				errors.Details(errE)["label"] = iid
				return errE
			}
		}
	}

	return nil
}
