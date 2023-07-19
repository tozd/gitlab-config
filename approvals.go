package config

import (
	"fmt"
	"net/http"
	"os"

	"github.com/xanzy/go-gitlab"
	"gitlab.com/tozd/go/errors"
)

// getApprovals populates configuration struct with GitLab's project's merge requests
// approvals available from GitLab approvals API endpoint.
func (c *GetCommand) getApprovals(client *gitlab.Client, configuration *Configuration) (bool, errors.E) {
	fmt.Fprintf(os.Stderr, "Getting approvals...\n")

	configuration.Approvals = map[string]interface{}{}

	descriptions, errE := getApprovalsDescriptions(c.DocsRef)
	if errE != nil {
		return false, errE
	}

	u := fmt.Sprintf("projects/%s/approvals", gitlab.PathEscape(c.Project))
	req, err := client.NewRequest(http.MethodGet, u, nil, nil)
	if err != nil {
		return false, errors.Wrapf(err, `failed to get approvals`)
	}

	approvals := map[string]interface{}{}

	_, err = client.Do(req, &approvals)
	if err != nil {
		return false, errors.Wrapf(err, `failed to get approvals`)
	}

	// Only retain those keys which can be edited through the API
	// (which are those available in descriptions).
	for key := range approvals {
		_, ok := descriptions[key]
		if !ok {
			delete(approvals, key)
		}
	}

	// Add comments for keys. We process these keys before writing YAML out.
	describeKeys(approvals, descriptions)

	configuration.Approvals = approvals

	return false, nil
}

// parseApprovalsDocumentation parses GitLab's documentation in Markdown for
// approvals API endpoint and extracts description of fields used to describe
// payload for project's merge requests approvals.
func parseApprovalsDocumentation(input []byte) (map[string]string, errors.E) {
	return parseTable(input, "Change configuration", nil)
}

// getApprovalsDescriptions obtains description of fields used to describe payload for
// project's merge requests approvals from GitLab's documentation for approvals API endpoint.
func getApprovalsDescriptions(gitRef string) (map[string]string, errors.E) {
	data, err := downloadFile(fmt.Sprintf("https://gitlab.com/gitlab-org/gitlab/-/raw/%s/doc/api/merge_request_approvals.md", gitRef))
	if err != nil {
		return nil, errors.Wrap(err, `failed to get approvals descriptions`)
	}
	return parseApprovalsDocumentation(data)
}

// updateApprovals updates GitLab project's merge requests approvals using GitLab
// approvals API endpoint based on the configuration struct.
func (c *SetCommand) updateApprovals(client *gitlab.Client, configuration *Configuration) errors.E {
	if configuration.Approvals == nil {
		return nil
	}

	fmt.Fprintf(os.Stderr, "Updating approvals...\n")

	u := fmt.Sprintf("projects/%s/approvals", gitlab.PathEscape(c.Project))
	req, err := client.NewRequest(http.MethodPost, u, configuration.Approvals, nil)
	if err != nil {
		return errors.Wrapf(err, `failed to update approvals`)
	}
	_, err = client.Do(req, nil)
	if err != nil {
		return errors.Wrapf(err, `failed to update approvals`)
	}

	return nil
}
