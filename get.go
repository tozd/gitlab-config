package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/xanzy/go-gitlab"
	"gitlab.com/tozd/go/errors"
)

const (
	fileMode = 0o600
)

// We do not use type=path for Output because we want a relative path.

// GetCommand describes parameters for the get command.
type GetCommand struct {
	GitLab

	Output     string `short:"o" placeholder:"PATH" default:".gitlab-conf.yml" help:"Where to save the configuration to. Can be \"-\" for stdout. Default is \"${default}\"."`                                                    //nolint:lll
	Avatar     string `short:"a" placeholder:"PATH" default:".gitlab-avatar.img" help:"Where to save the avatar to. File extension is set automatically. Default is \"${default}\"."`                                             //nolint:lll
	EncComment string `short:"E" placeholder:"STRING" default:"sops:enc" help:"Annotate sensitive values with the comment, marking them for encryption with SOPS. Set to an empty string to disable. Default is \"${default}\"."` //nolint:lll
	EncSuffix  string `short:"S" help:"Add the suffix to field names of sensitive values, marking them for encryption with SOPS. Disabled by default."`                                                                           //nolint:lll
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
	hasSensitive := false

	s, errE := c.getProject(client, &configuration)
	if errE != nil {
		return errE
	}
	hasSensitive = hasSensitive || s

	s, errE = c.getLabels(client, &configuration)
	if errE != nil {
		return errE
	}
	hasSensitive = hasSensitive || s

	s, errE = c.getProtectedBranches(client, &configuration)
	if errE != nil {
		return errE
	}
	hasSensitive = hasSensitive || s

	s, errE = c.getVariables(client, &configuration)
	if errE != nil {
		return errE
	}
	hasSensitive = hasSensitive || s

	data, errE := toConfigurationYAML(&configuration)
	if errE != nil {
		return errE
	}

	if c.Output != "-" {
		err = os.WriteFile(kong.ExpandPath(c.Output), data, fileMode)
	} else {
		_, err = os.Stdout.Write(data)
	}
	if err != nil {
		return errors.Wrapf(err, `cannot write configuration to "%s"`, c.Output)
	}

	fmt.Fprintf(os.Stderr, "Got everything.\n")
	if hasSensitive {
		args := []string{os.Args[0]}
		if globals.ChangeTo != "" {
			args = append(args, "-C", globals.ChangeTo)
		}
		// TODO: Remove "--". See: https://github.com/alecthomas/kong/issues/253
		args = append(args, "sops", "--", "--encrypt", "--mac-only-encrypted", "--in-place")
		if c.EncSuffix != "" {
			args = append(args, "--encrypted-suffix", c.EncSuffix)
		} else if c.EncComment != "" {
			args = append(args, "--encrypted-comment-regex", regexp.QuoteMeta(c.EncComment))
		}
		args = append(args, c.Output)
		fmt.Fprintf(os.Stderr, "WARNING: Configuration includes sensitive values. Consider encrypting the file. You can use SOPS, e.g.:\n  %s\n", strings.Join(args, " ")) //nolint:lll
	}

	return nil
}
