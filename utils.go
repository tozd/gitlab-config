package config

import (
	"net/url"
	"strings"
)

// pathEscape is a helper function to escape a project identifier.
func pathEscape(s string) string {
	return strings.ReplaceAll(url.PathEscape(s), ".", "%2E")
}
