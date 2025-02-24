package config

import (
	"fmt"
	"net/http"
	"os"
	"slices"
	"sort"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/xanzy/go-gitlab"
	"gitlab.com/tozd/go/errors"
)

// getProtectedTags populates configuration struct with configuration available
// from GitLab protected tags API endpoint.
func (c *GetCommand) getProtectedTags(client *gitlab.Client, configuration *Configuration) (bool, errors.E) { //nolint:unparam
	fmt.Fprintf(os.Stderr, "Getting protected tags...\n")

	configuration.ProtectedTags = []map[string]interface{}{}

	descriptions, errE := getProtectedTagsDescriptions(c.DocsRef)
	if errE != nil {
		return false, errE
	}
	// We need "name" later on.
	if _, ok := descriptions["name"]; !ok {
		return false, errors.New(`"name" field is missing in protected tags descriptions`)
	}
	configuration.ProtectedTagsComment = formatDescriptions(descriptions)

	u := fmt.Sprintf("projects/%s/protected_tags", gitlab.PathEscape(c.Project))
	options := &gitlab.ListProtectedTagsOptions{
		PerPage: maxGitLabPageSize,
		Page:    1,
	}

	for {
		req, err := client.NewRequest(http.MethodGet, u, options, nil)
		if err != nil {
			errE := errors.WithMessage(err, "failed to get protected tags")
			errors.Details(errE)["page"] = options.Page
			return false, errE
		}

		protectedTags := []map[string]interface{}{}

		response, err := client.Do(req, &protectedTags)
		if err != nil {
			errE := errors.WithMessage(err, "failed to get protected tags")
			errors.Details(errE)["page"] = options.Page
			return false, errE
		}

		if len(protectedTags) == 0 {
			break
		}

		for _, protectedTag := range protectedTags {
			// We rename to be consistent between getting and updating.
			protectedTag["allowed_to_create"] = protectedTag["create_access_levels"]

			// We for now remove ID because it is not useful for updating protected tags.
			// TODO: Use ID to just update protected tags.
			//       See: https://gitlab.com/tozd/gitlab/config/-/issues/18
			removeField(protectedTag["allowed_to_create"], "id")

			// Making sure ids and levels are an integer.
			castFloatsToInts(protectedTag)

			// Only retain those keys which can be edited through the API
			// (which are those available in descriptions).
			for key := range protectedTag {
				_, ok := descriptions[key]
				if !ok {
					delete(protectedTag, key)
				}
			}

			// Make the description be a comment for the sequence item.
			renameMapField(protectedTag, "access_level_description", "comment:")

			name, ok := protectedTag["name"]
			if !ok {
				return false, errors.New(`protected tag is missing field "name"`)
			}
			_, ok = name.(string)
			if !ok {
				errE := errors.New(`protected tag's field "name" is not a string`)
				errors.Details(errE)["type"] = fmt.Sprintf("%T", name)
				errors.Details(errE)["value"] = name
				return false, errE
			}

			configuration.ProtectedTags = append(configuration.ProtectedTags, protectedTag)
		}

		if response.NextPage == 0 {
			break
		}

		options.Page = response.NextPage
	}

	// We sort by protected tag's name so that we have deterministic order.
	sort.Slice(configuration.ProtectedTags, func(i, j int) bool {
		// We checked that name is string above.
		return configuration.ProtectedTags[i]["name"].(string) < configuration.ProtectedTags[j]["name"].(string) //nolint:forcetypeassert,errcheck
	})

	return false, nil
}

// parseProtectedTagsDocumentation parses GitLab's documentation in Markdown for
// protected tags API endpoint and extracts description of fields used to describe
// protected tags.
func parseProtectedTagsDocumentation(input []byte) (map[string]string, errors.E) {
	return parseTable(input, "Protect repository tags", func(key string) string {
		switch key {
		case "create_access_level":
			// We prefer that everything is done through "allowed_to_create".
			return ""
		default:
			return key
		}
	})
}

// getProtectedTagsDescriptions obtains description of fields used to describe
// an individual protected tags from GitLab's documentation for protected tags API endpoint.
func getProtectedTagsDescriptions(gitRef string) (map[string]string, errors.E) {
	data, err := downloadFile(fmt.Sprintf("https://gitlab.com/gitlab-org/gitlab/-/raw/%s/doc/api/protected_tags.md", gitRef))
	if err != nil {
		return nil, errors.WithMessage(err, "failed to get protected tags descriptions")
	}
	return parseProtectedTagsDocumentation(data)
}

// updateProtectedTags updates GitLab project's protected tags using GitLab
// protected tags API endpoint based on the configuration struct.
//
// It first unprotects all protected tags which the project does not have anymore
// configured as protected, and then updates or adds protection for configured
// protected tags. When updating an existing protected tag it briefly umprotects
// the tag and reprotects it with new configuration.
func (c *SetCommand) updateProtectedTags(client *gitlab.Client, configuration *Configuration) errors.E {
	if configuration.ProtectedTags == nil {
		return nil
	}

	fmt.Fprintf(os.Stderr, "Updating protected tags...\n")

	options := &gitlab.ListProtectedTagsOptions{
		PerPage: maxGitLabPageSize,
		Page:    1,
	}

	protectedTags := []*gitlab.ProtectedTag{}

	for {
		pt, response, err := client.ProtectedTags.ListProtectedTags(c.Project, options)
		if err != nil {
			errE := errors.WithMessage(err, "failed to get protected tags")
			errors.Details(errE)["page"] = options.Page
			return errE
		}

		protectedTags = append(protectedTags, pt...)

		if response.NextPage == 0 {
			break
		}

		options.Page = response.NextPage
	}

	existingProtectedTagsSet := mapset.NewThreadUnsafeSet[string]()
	for _, protectedTag := range protectedTags {
		existingProtectedTagsSet.Add(protectedTag.Name)
	}

	wantedProtectedTagsSet := mapset.NewThreadUnsafeSet[string]()
	for i, protectedTag := range configuration.ProtectedTags {
		name, ok := protectedTag["name"]
		if !ok {
			errE := errors.Errorf(`protected tag is missing field "name"`)
			errors.Details(errE)["index"] = i
			return errE
		}
		n, ok := name.(string)
		if !ok {
			errE := errors.New(`protected tag's field "name" is not a string`)
			errors.Details(errE)["index"] = i
			errors.Details(errE)["type"] = fmt.Sprintf("%T", name)
			errors.Details(errE)["value"] = name
			return errE
		}
		wantedProtectedTagsSet.Add(n)
	}

	extraProtectedTags := existingProtectedTagsSet.Difference(wantedProtectedTagsSet).ToSlice()
	slices.Sort(extraProtectedTags)
	for _, protectedTagName := range extraProtectedTags {
		_, err := client.ProtectedTags.UnprotectRepositoryTags(c.Project, protectedTagName)
		if err != nil {
			errE := errors.WithMessage(err, "failed to unprotect tag")
			errors.Details(errE)["tag"] = protectedTagName
			return errE
		}
	}

	u := fmt.Sprintf("projects/%s/protected_tags", gitlab.PathEscape(c.Project))

	for i, protectedTag := range configuration.ProtectedTags { //nolint:dupl
		// We made sure above that all protected tags in configuration have a string name.
		name := protectedTag["name"].(string) //nolint:errcheck,forcetypeassert

		// If project already have this protected tag, we have to
		// first unprotect it to be able to update the protected tag.
		if existingProtectedTagsSet.Contains(name) {
			_, err := client.ProtectedTags.UnprotectRepositoryTags(c.Project, name)
			if err != nil {
				errE := errors.WithMessage(err, "failed to unprotect tag before reprotecting")
				errors.Details(errE)["index"] = i
				errors.Details(errE)["tag"] = name
				return errE
			}
		}

		req, err := client.NewRequest(http.MethodPost, u, protectedTag, nil)
		if err != nil {
			errE := errors.WithMessage(err, "failed to protect tag")
			errors.Details(errE)["index"] = i
			errors.Details(errE)["tag"] = name
			return errE
		}
		_, err = client.Do(req, nil)
		if err != nil {
			errE := errors.WithMessage(err, "failed to protect tag")
			errors.Details(errE)["index"] = i
			errors.Details(errE)["tag"] = name
			return errE
		}
	}

	return nil
}
