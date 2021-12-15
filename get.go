package config

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/mitchellh/go-wordwrap"
	"github.com/xanzy/go-gitlab"
	"gitlab.com/tozd/go/errors"
	"gopkg.in/yaml.v3"
)

const (
	// See: https://docs.gitlab.com/ee/api/#offset-based-pagination
	maxGitLabPageSize = 100
	maxCommentWidth   = 80
	fileMode          = 0o600
	yamlIndent        = 2
)

// A reasonable subset of supported file extensions for avatar image.
// See: https://gitlab.com/gitlab-org/gitlab/-/blob/master/app/uploaders/avatar_uploader.rb
// See: https://gitlab.com/gitlab-org/gitlab/-/blob/master/lib/gitlab/file_type_detection.rb#L22
var avatarFileExtensions = []string{
	".png",
	".jpg",
	".jpeg",
	".gif",
	".ico",
}

// We do not use type=path for Output because we want a relative path.

// GetCommand describes parameters for the get command.
type GetCommand struct {
	GitLab

	Output string `short:"o" placeholder:"PATH" default:".gitlab-conf.yml" help:"Where to save the configuration to. Can be \"-\" for stdout. Default is \"${default}\"."`        //nolint:lll
	Avatar string `short:"a" placeholder:"PATH" default:".gitlab-avatar.img" help:"Where to save the avatar to. File extension is set automatically. Default is \"${default}\"."` //nolint:lll
}

// checkAvatarExtension returns an error if the provided file extension ext
// is not among allowed file extensions avatarFileExtensions.
func checkAvatarExtension(ext string) error {
	for _, valid := range avatarFileExtensions {
		if valid == ext {
			return nil
		}
	}
	return errors.Errorf(`invalid avatar extension "%s"`, ext)
}

// formatDescriptions formats descriptions to be used a comment block before a
// sequence of objects in YAML. The comment block describes fields of those
// objects.
func formatDescriptions(descriptions map[string]string) string {
	keys := []string{}
	for key := range descriptions {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	output := ""
	for _, key := range keys {
		description := key + ": " + descriptions[key] + "\n"
		output += wordwrap.WrapString(description, maxCommentWidth)
	}
	return output
}

// getProjectConfig populates configuration struct with configuration available
// from GitLab projects API endpoint.
func getProjectConfig(client *gitlab.Client, projectID, avatarPath string, configuration *Configuration) errors.E {
	descriptions, errE := getProjectConfigDescriptions()
	if errE != nil {
		return errE
	}

	u := fmt.Sprintf("projects/%s", gitlab.PathEscape(projectID))

	req, err := client.NewRequest(http.MethodGet, u, nil, nil)
	if err != nil {
		return errors.Wrap(err, `failed to get project`)
	}

	project := map[string]interface{}{}

	_, err = client.Do(req, &project)
	if err != nil {
		return errors.Wrap(err, `failed to get project`)
	}

	// We use a separate top-level configuration for avatar instead.
	avatarURL, ok := project["avatar_url"]
	if ok && avatarURL != nil {
		avatarURL, ok := avatarURL.(string) //nolint:govet
		if !ok {
			return errors.New(`invalid "avatar_url"`)
		}
		avatarExt := path.Ext(avatarURL)
		err := checkAvatarExtension(avatarExt)
		if err != nil {
			return errors.Wrapf(err, `invalid "avatar_url": %s`, avatarURL)
		}
		// TODO: Make this work for private avatars, too.
		//       See: https://gitlab.com/gitlab-org/gitlab/-/issues/25498
		avatar, err := downloadFile(avatarURL)
		if err != nil {
			return errors.Wrapf(err, `failed to get project avatar from "%s"`, avatarURL)
		}
		avatarPath = strings.TrimSuffix(avatarPath, path.Ext(avatarPath)) + avatarExt
		err = os.WriteFile(avatarPath, avatar, fileMode)
		if err != nil {
			return errors.Wrapf(err, `failed to save avatar to "%s"`, avatarPath)
		}
		configuration.Avatar = avatarPath
	}

	// We use a separate top-level configuration for shared with groups instead.
	sharedWithGroups, ok := project["shared_with_groups"]
	if ok && sharedWithGroups != nil {
		sharedWithGroups, ok := sharedWithGroups.([]interface{}) //nolint:govet
		if !ok {
			return errors.New(`invalid "shared_with_groups"`)
		}
		if len(sharedWithGroups) > 0 {
			configuration.SharedWithGroups = []map[string]interface{}{}
			shareDescriptions, err := getShareProjectDescriptions()
			if err != nil {
				return err
			}
			for i, sharedWithGroup := range sharedWithGroups {
				sharedWithGroup, ok := sharedWithGroup.(map[string]interface{})
				if !ok {
					return errors.Errorf(`invalid "shared_with_groups" at index %d`, i)
				}
				groupFullPath := sharedWithGroup["group_full_path"]
				// Rename because share API has a different key than get project API.
				sharedWithGroup["group_access"] = sharedWithGroup["group_access_level"]
				// Making sure it is an integer.
				sharedWithGroup["group_id"] = int(sharedWithGroup["group_id"].(float64))

				// Only retain those keys which can be edited through the share API
				// (which are those available in descriptions).
				for key := range sharedWithGroup {
					_, ok = shareDescriptions[key]
					if !ok {
						delete(sharedWithGroup, key)
					}
				}

				// Add comment for the sequence item itself.
				if groupFullPath != nil {
					sharedWithGroup["comment:"] = groupFullPath
				}

				configuration.SharedWithGroups = append(configuration.SharedWithGroups, sharedWithGroup)
			}
			configuration.SharedWithGroupsComment = formatDescriptions(shareDescriptions)
		}
	}

	// We use a separate top-level configuration for fork relationship.
	forkedFromProject, ok := project["forked_from_project"]
	if ok && forkedFromProject != nil {
		forkedFromProject, ok := forkedFromProject.(map[string]interface{})
		if !ok {
			return errors.New(`invalid "forked_from_project"`)
		}
		forkID, ok := forkedFromProject["id"]
		if ok {
			// Making sure it is an integer.
			configuration.ForkedFromProject = int(forkID.(float64))
			forkPathWithNamespace := forkedFromProject["path_with_namespace"]
			if forkPathWithNamespace != nil {
				configuration.ForkedFromProjectComment, ok = forkPathWithNamespace.(string)
				if !ok {
					return errors.New(`invalid "path_with_namespace" in "forked_from_project"`)
				}
			}
		}
	}

	// Only retain those keys which can be edited through the edit API
	// (which are those available in descriptions). We cannot add comments
	// at the same time because we might delete them, too, because they are
	// not found in descriptions.
	for key := range project {
		_, ok := descriptions[key]
		if !ok {
			delete(project, key)
		}
	}

	// This cannot be configured simply through the edit API, this just enabled/disables it.
	// We use a separate top-level configuration for mirroring instead.
	delete(project, "mirror")

	// Remove deprecated name_regex key in favor of new name_regex_delete.
	if project["container_expiration_policy"] != nil {
		policy, ok := project["container_expiration_policy"].(map[string]interface{})
		if !ok {
			return errors.New(`invalid "container_expiration_policy"`)
		}
		if policy["name_regex"] != nil && policy["name_regex_delete"] == nil {
			policy["name_regex_delete"] = policy["name_regex"]
			delete(policy, "name_regex")
		} else if policy["name_regex"] != nil && policy["name_regex_delete"] != nil {
			delete(policy, "name_regex")
		}

		// It is not an editable key.
		delete(policy, "next_run_at")
	}

	// Add comments for keys. We process these keys before writing YAML out.
	for key := range project {
		project["comment:"+key] = descriptions[key]
	}

	configuration.Project = project

	return nil
}

// getProjectLabels populates configuration struct with configuration available
// from GitLab labels API endpoint.
func getProjectLabels(client *gitlab.Client, projectID string, configuration *Configuration) errors.E {
	descriptions, errE := getProjectLabelsDescriptions()
	if errE != nil {
		return errE
	}

	u := fmt.Sprintf("projects/%s/labels", gitlab.PathEscape(projectID))
	options := &gitlab.ListLabelsOptions{ //nolint:exhaustivestruct
		ListOptions: gitlab.ListOptions{
			PerPage: maxGitLabPageSize,
			Page:    1,
		},
		IncludeAncestorGroups: gitlab.Bool(false),
	}

	for {
		req, err := client.NewRequest(http.MethodGet, u, options, nil)
		if err != nil {
			return errors.Wrapf(err, `failed to get project labels, page %d`, options.Page)
		}

		labels := []map[string]interface{}{}

		response, err := client.Do(req, &labels)
		if err != nil {
			return errors.Wrapf(err, `failed to get project labels, page %d`, options.Page)
		}

		if len(labels) == 0 {
			break
		}

		if configuration.Labels == nil {
			configuration.Labels = []map[string]interface{}{}
		}

		for _, label := range labels {
			// Making sure it is an integer.
			label["id"] = int(label["id"].(float64))

			// Only retain those keys which can be edited through the share API
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

	configuration.LabelsComment = formatDescriptions(descriptions)

	return nil
}

// downloadFile downloads a file from url URL.
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

	// TODO: Handle errors better.
	//       On error this tries to parse the error response as API error, which fails for arbitrary HTTP requests.
	_, err = client.Do(req, &buffer)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return buffer.Bytes(), nil
}

// getProjectConfigDescriptions obtains description of fields used to describe
// an individual project from GitLab's documentation for projects API endpoint.
func getProjectConfigDescriptions() (map[string]string, errors.E) {
	data, err := downloadFile("https://gitlab.com/gitlab-org/gitlab/-/raw/master/doc/api/projects.md")
	if err != nil {
		return nil, errors.Wrap(err, `failed to get project configuration descriptions`)
	}
	return parseProjectDocumentation(data)
}

// getShareProjectDescriptions obtains description of fields used to describe payload for
// sharing a project with a group from GitLab's documentation for projects API endpoint.
func getShareProjectDescriptions() (map[string]string, errors.E) {
	data, err := downloadFile("https://gitlab.com/gitlab-org/gitlab/-/raw/master/doc/api/projects.md")
	if err != nil {
		return nil, errors.Wrap(err, `failed to get share project descriptions`)
	}
	return parseShareDocumentation(data)
}

// getProjectLabelsDescriptions obtains description of fields used to describe
// an individual label from GitLab's documentation for labels API endpoint.
func getProjectLabelsDescriptions() (map[string]string, errors.E) {
	data, err := downloadFile("https://gitlab.com/gitlab-org/gitlab/-/raw/master/doc/api/labels.md")
	if err != nil {
		return nil, errors.Wrap(err, `failed to get project labels descriptions`)
	}
	return parseLabelsDocumentation(data)
}

// saveConfiguration saves configuration to output file path in YAML.
// output can be "-" to save it to stdout.
//
// Saved YAML contains configuration comments.
func saveConfiguration(configuration *Configuration, output string) errors.E {
	var node yaml.Node
	err := (&node).Encode(configuration)
	if err != nil {
		return errors.Wrap(err, `cannot encode configuration`)
	}
	return writeYAML(&node, output)
}

// setYAMLComments modifies YAML node by moving comments in children nodes which have
// "comment:" prefix in object field names to corresponding data fields (and their nodes).
func setYAMLComments(node *yaml.Node) {
	if node.Kind != yaml.MappingNode {
		for _, content := range node.Content {
			setYAMLComments(content)
		}
		return
	}

	// We first extract all comments.
	comments := map[string]string{}
	contentsToDelete := []int{}
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i].Value
		if strings.HasPrefix(key, "comment:") {
			comments[strings.TrimPrefix(key, "comment:")] = node.Content[i+1].Value
			contentsToDelete = append(contentsToDelete, i, i+1)
		}
	}

	// We iterate in the reverse order.
	for i := len(contentsToDelete) - 1; i >= 0; i-- {
		k := contentsToDelete[i]
		// Remove one content node after the other.
		node.Content = append(node.Content[:k], node.Content[k+1:]...)
	}

	// Finally set comments.
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i].Value
		comment, ok := comments[key]
		// Only if there is a comment and another comment is not already set.
		if ok && comment != "" && node.Content[i].HeadComment == "" {
			node.Content[i].HeadComment = wordwrap.WrapString(comment, maxCommentWidth)
		}

		// And recurse at the same time.
		setYAMLComments(node.Content[i+1])
	}

	// Set comment for the node itself.
	comment, ok := comments[""]
	// Only if there is a comment and another comment is not already set.
	if ok && comment != "" && node.HeadComment == "" {
		node.HeadComment = wordwrap.WrapString(comment, maxCommentWidth)
	}
}

// writeYAML writes YAML node to output file path.
// output can be "-" to save it to stdout.
//
// Comments in the YAML node are written out as well.
func writeYAML(node *yaml.Node, output string) errors.E {
	setYAMLComments(node)

	buffer := bytes.Buffer{}

	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(yamlIndent)
	err := encoder.Encode(node)
	if err != nil {
		return errors.Wrap(err, `cannot marshal configuration`)
	}
	err = encoder.Close()
	if err != nil {
		return errors.Wrap(err, `cannot marshal configuration`)
	}

	if output != "-" {
		err = os.WriteFile(kong.ExpandPath(output), buffer.Bytes(), fileMode)
	} else {
		_, err = os.Stdout.Write(buffer.Bytes())
	}
	if err != nil {
		return errors.Wrapf(err, `cannot write configuration to "%s"`, output)
	}

	return nil
}

// Run runs the get command.
func (c *GetCommand) Run(globals *Globals) errors.E {
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

	client, err := gitlab.NewClient(c.Token, gitlab.WithBaseURL(c.BaseURL))
	if err != nil {
		return errors.Wrap(err, `failed to create GitLab API client instance`)
	}

	var configuration Configuration

	errE := getProjectConfig(client, c.Project, c.Avatar, &configuration)
	if errE != nil {
		return errE
	}

	errE = getProjectLabels(client, c.Project, &configuration)
	if errE != nil {
		return errE
	}

	return saveConfiguration(&configuration, c.Output)
}
