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

// See: https://docs.gitlab.com/ee/api/#offset-based-pagination
const maxGitLabPageSize = 100

// A reasonable subset of supported file extensions for avatar image.
// See: https://gitlab.com/gitlab-org/gitlab/-/blob/master/app/uploaders/avatar_uploader.rb
var avatarFileExtensions = []string{
	".png",
	".jpg",
	".jpeg",
	".gif",
	".ico",
}

// We do not use type=path for Output because we want a relative path.
type SaveCommand struct {
	GitLab

	Output string `short:"o" placeholder:"PATH" default:".gitlab-conf.yml" help:"Where to save the configuration to. Can be \"-\" for stdout. Default is \"${default}\"."`
	Avatar string `short:"a" placeholder:"PATH" default:".gitlab-avatar.img" help:"Where to save the avatar to. File extension is set automatically. Default is \"${default}\"."`
}

func checkAvatarExtension(ext string) error {
	for _, valid := range avatarFileExtensions {
		if valid == ext {
			return nil
		}
	}
	return errors.Errorf(`invalid avatar extension "%s"`, ext)
}

func formatDescriptions(descriptions map[string]string) string {
	keys := []string{}
	for key := range descriptions {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	output := ""
	for _, key := range keys {
		description := key + ": " + descriptions[key] + "\n"
		output += wordwrap.WrapString(description, 80)
	}
	return output
}

func getProjectConfig(client *gitlab.Client, projectID, avatarPath string, configuration *Configuration) errors.E {
	descriptions, errE := getProjectConfigDescriptions()
	if errE != nil {
		return errE
	}

	u := fmt.Sprintf("projects/%s", pathEscape(projectID))

	req, err := client.NewRequest(http.MethodGet, u, nil, nil)
	if err != nil {
		return errors.Wrap(err, `failed to get GitLab project`)
	}

	project := map[string]interface{}{}

	_, err = client.Do(req, &project)
	if err != nil {
		return errors.Wrap(err, `failed to get GitLab project`)
	}

	// We use a separate top-level configuration for avatar instead.
	avatarUrl, ok := project["avatar_url"]
	if ok && avatarUrl != nil {
		avatarUrl := avatarUrl.(string)
		avatarExt := path.Ext(avatarUrl)
		err := checkAvatarExtension(avatarExt)
		if err != nil {
			return errors.Wrapf(err, `invalid avatar URL "%s"`, avatarUrl)
		}
		// TODO: Make this work for private avatars, too.
		//       See: https://gitlab.com/gitlab-org/gitlab/-/issues/25498
		avatar, err := downloadFile(avatarUrl)
		if err != nil {
			return errors.Wrapf(err, `failed to get GitLab project avatar from "%s"`, avatarUrl)
		}
		avatarPath = strings.TrimSuffix(avatarPath, path.Ext(avatarPath)) + avatarExt
		err = os.WriteFile(avatarPath, avatar, 0o644)
		if err != nil {
			return errors.Wrapf(err, `failed to save avatar to "%s"`, avatarPath)
		}
		configuration.Avatar = avatarPath
	}

	// We use a separate top-level configuration for shared with groups instead.
	sharedWithGroups, ok := project["shared_with_groups"]
	if ok && sharedWithGroups != nil {
		sharedWithGroups := sharedWithGroups.([]interface{})
		if len(sharedWithGroups) > 0 {
			configuration.SharedWithGroups = []map[string]interface{}{}
			shareDescriptions, err := getShareProjectDescriptions()
			if err != nil {
				return err
			}
			for _, sharedWithGroup := range sharedWithGroups {
				sharedWithGroup := sharedWithGroup.(map[string]interface{})
				groupFullPath := sharedWithGroup["group_full_path"]
				// Rename because share API has a different key than get project API.
				sharedWithGroup["group_access"] = sharedWithGroup["group_access_level"]
				// Making sure it is an integer.
				sharedWithGroup["group_id"] = int(sharedWithGroup["group_id"].(float64))

				// Only retain those keys which can be edited through the share API
				// (which are those available in descriptions).
				for key := range sharedWithGroup {
					_, ok := shareDescriptions[key]
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
		forkedFromProject := forkedFromProject.(map[string]interface{})
		forkID, ok := forkedFromProject["id"]
		if ok {
			// Making sure it is an integer.
			configuration.ForkedFromProject = int(forkID.(float64))
			forkPathWithNamespace := forkedFromProject["path_with_namespace"]
			if forkPathWithNamespace != nil {
				configuration.ForkedFromProjectComment = forkPathWithNamespace.(string)
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
		container_expiration_policy := project["container_expiration_policy"].(map[string]interface{})
		if container_expiration_policy["name_regex"] != nil && container_expiration_policy["name_regex_delete"] == nil {
			container_expiration_policy["name_regex_delete"] = container_expiration_policy["name_regex"]
			delete(container_expiration_policy, "name_regex")
		} else if container_expiration_policy["name_regex"] != nil && container_expiration_policy["name_regex_delete"] != nil {
			delete(container_expiration_policy, "name_regex")
		}
	}

	// Add comments for keys. We process these keys before writing YAML out.
	for key := range project {
		project["comment:"+key] = descriptions[key]
	}

	configuration.Project = project

	return nil
}

func getProjectLabels(client *gitlab.Client, projectID string, configuration *Configuration) errors.E {
	descriptions, errE := getProjectLabelsDescriptions()
	if errE != nil {
		return errE
	}

	u := fmt.Sprintf("projects/%s/labels", pathEscape(projectID))
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
			return errors.Wrapf(err, `failed to get GitLab project labels, page %d`, options.Page)
		}

		labels := []map[string]interface{}{}

		response, err := client.Do(req, &labels)
		if err != nil {
			return errors.Wrapf(err, `failed to get GitLab project labels, page %d`, options.Page)
		}

		if len(labels) == 0 {
			break
		}

		if configuration.Labels == nil {
			configuration.Labels = []map[string]interface{}{}
		}

		for _, label := range labels {
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

	configuration.LabelsComment = formatDescriptions(descriptions)

	return nil
}

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

	// TODO: On error this tries to parse the error response as API error, which fails for arbitrary HTTP requests. Do something else?
	_, err = client.Do(req, &buffer)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return buffer.Bytes(), nil
}

func getProjectConfigDescriptions() (map[string]string, errors.E) {
	data, err := downloadFile("https://gitlab.com/gitlab-org/gitlab/-/raw/master/doc/api/projects.md")
	if err != nil {
		return nil, errors.Wrap(err, `failed to get GitLab project configuration descriptions`)
	}
	return parseProjectTable(data)
}

func getShareProjectDescriptions() (map[string]string, errors.E) {
	data, err := downloadFile("https://gitlab.com/gitlab-org/gitlab/-/raw/master/doc/api/projects.md")
	if err != nil {
		return nil, errors.Wrap(err, `failed to get GitLab share project descriptions`)
	}
	return parseShareTable(data)
}

func getProjectLabelsDescriptions() (map[string]string, errors.E) {
	data, err := downloadFile("https://gitlab.com/gitlab-org/gitlab/-/raw/master/doc/api/labels.md")
	if err != nil {
		return nil, errors.Wrap(err, `failed to get GitLab project labels descriptions`)
	}
	return parseLabelsTable(data)
}

func saveConfiguration(configuration *Configuration, output string) errors.E {
	node := &yaml.Node{}
	err := node.Encode(configuration)
	if err != nil {
		return errors.Wrap(err, `cannot encode configuration`)
	}
	return writeYAML(node, output)
}

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
			node.Content[i].HeadComment = wordwrap.WrapString(comment, 80)
		}

		// And recurse at the same time.
		setYAMLComments(node.Content[i+1])
	}

	// Set comment for the node itself.
	comment, ok := comments[""]
	// Only if there is a comment and another comment is not already set.
	if ok && comment != "" && node.HeadComment == "" {
		node.HeadComment = wordwrap.WrapString(comment, 80)
	}
}

func writeYAML(node *yaml.Node, output string) errors.E {
	setYAMLComments(node)

	buffer := bytes.Buffer{}

	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(2)
	err := encoder.Encode(node)
	if err != nil {
		return errors.Wrap(err, `cannot marshal configuration`)
	}
	err = encoder.Close()
	if err != nil {
		return errors.Wrap(err, `cannot marshal configuration`)
	}

	if output != "-" {
		err = os.WriteFile(kong.ExpandPath(output), buffer.Bytes(), 0o644)
	} else {
		_, err = os.Stdout.Write(buffer.Bytes())
	}
	if err != nil {
		return errors.Wrapf(err, `cannot write configuration to "%s"`, output)
	}

	return nil
}

func (c *SaveCommand) Run(globals *Globals) errors.E {
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

	configuration := Configuration{}

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
