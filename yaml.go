package config

import (
	"bytes"
	"sort"
	"strings"

	"github.com/mitchellh/go-wordwrap"
	"gitlab.com/tozd/go/errors"
	"gopkg.in/yaml.v3"
)

const (
	maxCommentWidth = 80
	yamlIndent      = 2
)

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

// toConfigurationYAML returns configuration as YAML.
//
// YAML contains configuration comments.
func toConfigurationYAML(configuration *Configuration) ([]byte, errors.E) {
	var node yaml.Node
	err := (&node).Encode(configuration)
	if err != nil {
		return nil, errors.Wrap(err, `cannot encode configuration`)
	}
	return toYAML(&node)
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

// toYAML converts YAML node to bytes.
//
// Comments in the YAML node are converted as well.
func toYAML(node *yaml.Node) ([]byte, errors.E) {
	setYAMLComments(node)

	buffer := bytes.Buffer{}

	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(yamlIndent)
	err := encoder.Encode(node)
	if err != nil {
		return nil, errors.Wrap(err, `cannot marshal configuration`)
	}
	err = encoder.Close()
	if err != nil {
		return nil, errors.Wrap(err, `cannot marshal configuration`)
	}

	return buffer.Bytes(), nil
}
