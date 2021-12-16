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
