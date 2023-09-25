package config

import (
	"bytes"
	"net/http"
	"strings"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/xanzy/go-gitlab"
	"gitlab.com/tozd/go/errors"
)

const (
	// See: https://docs.gitlab.com/ee/api/#offset-based-pagination
	maxGitLabPageSize = 100
)

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

// renameAnyField renames field named "from" to "to" anywhere in the arbitrary input
// structure, even if it is nested inside other maps or slices.
func renameAnyField(input interface{}, from, to string) {
	switch in := input.(type) {
	case []interface{}:
		for _, v := range in {
			renameAnyField(v, from, to)
		}
	case []map[string]interface{}:
		for _, v := range in {
			renameAnyField(v, from, to)
		}
	case map[string]interface{}:
		renameMapField(in, from, to)
	}
}

// renameMapField renames field named "from" to "to" anywhere in the map input
// structure, even if it is nested inside other maps or slices.
func renameMapField(input map[string]interface{}, from, to string) {
	for key, value := range input {
		renameAnyField(value, from, to)

		if key == from {
			input[to] = value
			delete(input, key)
		}
	}
}

// removeField removes field named "field" anywhere in the arbitrary input
// structure, even if it is nested inside other maps or slices.
func removeField(input interface{}, field string) {
	switch in := input.(type) {
	case []interface{}:
		for _, v := range in {
			removeField(v, field)
		}
	case []map[string]interface{}:
		for _, v := range in {
			removeField(v, field)
		}
	case map[string]interface{}:
		for key, value := range in {
			removeField(value, field)

			if key == field {
				delete(in, key)
			}
		}
	}
}

// removeFieldSuffix removes suffix from field names anywhere in the arbitrary
// input structure, even if it is nested inside other maps or slices.
func removeFieldSuffix(input interface{}, suffix string) {
	if suffix == "" {
		return
	}

	switch in := input.(type) {
	case []interface{}:
		for _, v := range in {
			removeFieldSuffix(v, suffix)
		}
	case []map[string]interface{}:
		for _, v := range in {
			removeFieldSuffix(v, suffix)
		}
	case map[string]interface{}:
		for key, value := range in {
			removeFieldSuffix(value, suffix)

			if strings.HasSuffix(key, suffix) {
				in[strings.TrimSuffix(key, suffix)] = value
				delete(in, key)
			}
		}
	}
}

// castFloatsToInts casts all floats to ints anywhere in the arbitrary
// input structure, even if it is nested inside other maps or slices.
func castFloatsToInts(input interface{}) {
	switch in := input.(type) {
	case []interface{}:
		for i, v := range in {
			switch n := v.(type) {
			case float64:
				in[i] = int(n)
			default:
				castFloatsToInts(v)
			}
		}
	case []map[string]interface{}:
		for _, v := range in {
			castFloatsToInts(v)
		}
	case map[string]interface{}:
		for key, value := range in {
			switch n := value.(type) {
			case float64:
				in[key] = int(n)
			default:
				castFloatsToInts(value)
			}
		}
	}
}

// describeKeys adds comments for all keys in obj found in descriptions.
func describeKeys(obj map[string]interface{}, descriptions map[string]string) {
	// We first make a copy of all existing keys and then add descriptions.
	// This prevents us from trying to add descriptions for the (comment) keys
	// we just added.
	keys := []string{}
	for key := range obj {
		keys = append(keys, key)
	}
	for _, key := range keys {
		d, ok := descriptions[key]
		if ok {
			obj["comment:"+key] = d
		}
	}
}

// convertNestedObjectsToIds converts a slice of objects to a slice of
// IDs from their "id" fields. If objects have "name" field, the value
// is set as a comment before the ID.
func convertNestedObjectsToIds(input interface{}) ([]interface{}, error) {
	ids := []interface{}{}

	if input == nil {
		return ids, nil
	}

	list, ok := input.([]interface{})
	if !ok {
		return nil, errors.New("not a list")
	}

	for i, element := range list {
		el, ok := element.(map[string]interface{})
		if !ok {
			errE := errors.New("not an object")
			errors.Details(errE)["index"] = i
			return nil, errE
		}

		name, ok := el["name"]
		if ok {
			n, ok := name.(string) //nolint:govet
			if !ok {
				errE := errors.New(`"name" not a string`)
				errors.Details(errE)["index"] = i
				return nil, errE
			}
			ids = append(ids, "comment:"+n)
		}

		id, ok := el["id"]
		if !ok {
			errE := errors.New(`"id" is missing`)
			errors.Details(errE)["index"] = i
			return nil, errE
		}
		ids = append(ids, id)
	}

	return ids, nil
}
