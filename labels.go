package config

import (
	"fmt"
	"net/http"
	"os"
	"sort"

	mapset "github.com/deckarep/golang-set"
	"github.com/xanzy/go-gitlab"
	"gitlab.com/tozd/go/errors"
)

// getLabels populates configuration struct with configuration available
// from GitLab labels API endpoint.
func (c *GetCommand) getLabels(client *gitlab.Client, configuration *Configuration) (bool, errors.E) {
	fmt.Fprintf(os.Stderr, "Getting labels...\n")

	configuration.Labels = []map[string]interface{}{}

	descriptions, errE := getLabelsDescriptions(c.DocsRef)
	if errE != nil {
		return false, errE
	}
	configuration.LabelsComment = formatDescriptions(descriptions)

	u := fmt.Sprintf("projects/%s/labels", gitlab.PathEscape(c.Project))
	options := &gitlab.ListLabelsOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: maxGitLabPageSize,
			Page:    1,
		},
		IncludeAncestorGroups: gitlab.Bool(false),
	}

	for {
		req, err := client.NewRequest(http.MethodGet, u, options, nil)
		if err != nil {
			return false, errors.Wrapf(err, `failed to get project labels, page %d`, options.Page)
		}

		labels := []map[string]interface{}{}

		response, err := client.Do(req, &labels)
		if err != nil {
			return false, errors.Wrapf(err, `failed to get project labels, page %d`, options.Page)
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

			configuration.Labels = append(configuration.Labels, label)
		}

		if response.NextPage == 0 {
			break
		}

		options.Page = response.NextPage
	}

	// We sort by label ID so that we have deterministic order.
	sort.Slice(configuration.Labels, func(i, j int) bool {
		return configuration.Labels[i]["id"].(int) < configuration.Labels[j]["id"].(int)
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
		return nil, errors.Wrap(err, `failed to get project labels descriptions`)
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

	options := &gitlab.ListLabelsOptions{
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
			return errors.Wrapf(err, `failed to get project labels, page %d`, options.Page)
		}

		labels = append(labels, ls...)

		if response.NextPage == 0 {
			break
		}

		options.Page = response.NextPage
	}

	existingLabelsSet := mapset.NewThreadUnsafeSet()
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
			id, ok := id.(int) //nolint:govet
			if !ok {
				return errors.Errorf(`invalid "id" in "labels" at index %d`, i)
			}
			if existingLabelsSet.Contains(id) {
				continue
			}
			// Label does not exist with that ID. We remove the ID and leave to name matching to
			// find the correct ID, if it exists. Otherwise we will just create a new lable.
			delete(label, "id")
		}

		name, ok := label["name"]
		if !ok {
			return errors.Errorf(`label in configuration at index %d does not have "name"`, i)
		}
		id, ok = namesToIDs[name.(string)]
		if ok {
			label["id"] = id
		}
	}

	wantedLabelsSet := mapset.NewThreadUnsafeSet()
	for _, label := range configuration.Labels {
		id, ok := label["id"]
		if ok {
			wantedLabelsSet.Add(id.(int))
		}
	}

	extraLabelsSet := existingLabelsSet.Difference(wantedLabelsSet)
	for _, extraLabel := range extraLabelsSet.ToSlice() {
		labelID := extraLabel.(int) //nolint:errcheck
		// TODO: Use go-gitlab's function once it is updated to new API.
		//       See: https://github.com/xanzy/go-gitlab/issues/1321
		u := fmt.Sprintf("projects/%s/labels/%d", gitlab.PathEscape(c.Project), labelID)
		req, err := client.NewRequest(http.MethodDelete, u, nil, nil)
		if err != nil {
			return errors.Wrapf(err, `failed to delete label %d`, labelID)
		}
		_, err = client.Do(req, nil)
		if err != nil {
			return errors.Wrapf(err, `failed to delete label %d`, labelID)
		}
	}

	for _, label := range configuration.Labels {
		id, ok := label["id"]
		if !ok {
			u := fmt.Sprintf("projects/%s/labels", gitlab.PathEscape(c.Project))
			req, err := client.NewRequest(http.MethodPost, u, label, nil)
			if err != nil {
				// We made sure above that all labels in configuration without label ID have name.
				return errors.Wrapf(err, `failed to create label "%s"`, label["name"].(string))
			}
			_, err = client.Do(req, nil)
			if err != nil {
				// We made sure above that all labels in configuration without label ID have name.
				return errors.Wrapf(err, `failed to create label "%s"`, label["name"].(string))
			}
		} else {
			// We made sure above that all labels in configuration with label ID exist
			// and that they are ints.
			id := id.(int) //nolint:errcheck
			u := fmt.Sprintf("projects/%s/labels/%d", gitlab.PathEscape(c.Project), id)
			req, err := client.NewRequest(http.MethodPut, u, label, nil)
			if err != nil {
				return errors.Wrapf(err, `failed to update label %d`, id)
			}
			_, err = client.Do(req, nil)
			if err != nil {
				return errors.Wrapf(err, `failed to update label "%d`, id)
			}
		}
	}

	return nil
}
