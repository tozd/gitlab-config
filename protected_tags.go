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
		return false, errors.New(`"name" missing in protected tags descriptions`)
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
			return false, errors.Wrapf(err, `failed to get protected tags, page %d`, options.Page)
		}

		protectedTags := []map[string]interface{}{}

		response, err := client.Do(req, &protectedTags)
		if err != nil {
			return false, errors.Wrapf(err, `failed to get protected tags, page %d`, options.Page)
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
				return false, errors.Errorf(`protected tag is missing "name"`)
			}
			_, ok = name.(string)
			if !ok {
				return false, errors.Errorf(`protected tag "name" is not a string, but %T: %s`, name, name)
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
		// We checked that id is int above.
		return configuration.ProtectedTags[i]["name"].(string) < configuration.ProtectedTags[j]["name"].(string) //nolint:forcetypeassert
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
		return nil, errors.Wrap(err, `failed to get protected tags descriptions`)
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
			return errors.Wrapf(err, `failed to get protected tags, page %d`, options.Page)
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
			return errors.Errorf(`protected tag in configuration at index %d does not have "name"`, i)
		}
		n, ok := name.(string)
		if !ok {
			return errors.Errorf(`protected tags "name" at index %d is not a string, but %T: %s`, i, name, name)
		}
		wantedProtectedTagsSet.Add(n)
	}

	extraProtectedTagsSet := existingProtectedTagsSet.Difference(wantedProtectedTagsSet)
	for _, protectedTagName := range extraProtectedTagsSet.ToSlice() {
		_, err := client.ProtectedTags.UnprotectRepositoryTags(c.Project, protectedTagName)
		if err != nil {
			return errors.Wrapf(err, `failed to unprotect tag "%s"`, protectedTagName)
		}
	}

	u := fmt.Sprintf("projects/%s/protected_tags", gitlab.PathEscape(c.Project))

	for _, protectedTag := range configuration.ProtectedTags {
		// We made sure above that all protected tags in configuration have a string name.
		name := protectedTag["name"].(string) //nolint:errcheck,forcetypeassert

		// If project already have this protected tag, we have to
		// first unprotect it to be able to update the protected tag.
		if existingProtectedTagsSet.Contains(name) {
			_, err := client.ProtectedTags.UnprotectRepositoryTags(c.Project, name)
			if err != nil {
				return errors.Wrapf(err, `failed to unprotect tag "%s" before reprotecting`, name)
			}
		}

		req, err := client.NewRequest(http.MethodPost, u, protectedTag, nil)
		if err != nil {
			return errors.Wrapf(err, `failed to protect tag "%s"`, name)
		}
		_, err = client.Do(req, nil)
		if err != nil {
			return errors.Wrapf(err, `failed to protect tag "%s"`, name)
		}
	}

	return nil
}
