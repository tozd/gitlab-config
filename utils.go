package config

import (
	"bytes"
	"net/http"

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

// renameAnyField renames field named from to to anywhere in the arbitrary input
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

// renameMapField renames field named from to to anywhere in the map input
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
