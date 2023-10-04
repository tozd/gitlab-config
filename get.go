package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/xanzy/go-gitlab"
	"gitlab.com/tozd/go/errors"
	"gitlab.com/tozd/go/x"
)

const (
	fileMode = 0o600
)

// We do not use type=path for Output because we want a relative path.

// GetCommand describes parameters for the get command.
type GetCommand struct {
	GitLab

	Output     string `default:".gitlab-conf.yml"                                                                                            help:"Where to save the configuration to. Can be \"-\" for stdout. Default is \"${default}\"."                                                          placeholder:"PATH"   short:"o"` //nolint:lll
	Avatar     string `default:".gitlab-avatar.img"                                                                                          help:"Where to save the avatar to. File extension is set automatically. Default is \"${default}\"."                                                     placeholder:"PATH"   short:"a"` //nolint:lll
	EncComment string `default:"sops:enc"                                                                                                    help:"Annotate sensitive values with the comment, marking them for encryption with SOPS. Set to an empty string to disable. Default is \"${default}\"." placeholder:"STRING" short:"E"` //nolint:lll
	EncSuffix  string `help:"Add the suffix to field names of sensitive values, marking them for encryption with SOPS. Disabled by default." short:"S"`                                                                                                                                                                              //nolint:lll
}

// Run runs the get command.
func (c *GetCommand) Run(globals *Globals) errors.E {
	if c.Project == "" {
		projectID, errE := x.InferGitLabProjectID(".")
		if errE != nil {
			return errE
		}
		c.Project = projectID
	}

	client, err := gitlab.NewClient(c.Token, gitlab.WithBaseURL(c.BaseURL))
	if err != nil {
		return errors.WithMessage(err, "failed to create GitLab API client instance")
	}

	var configuration Configuration
	hasSensitive := false

	s, errE := c.getProject(client, &configuration)
	if errE != nil {
		return errE
	}
	hasSensitive = hasSensitive || s

	s, errE = c.getApprovals(client, &configuration)
	if errE != nil {
		return errE
	}
	hasSensitive = hasSensitive || s

	s, errE = c.getApprovalRules(client, &configuration)
	if errE != nil {
		return errE
	}
	hasSensitive = hasSensitive || s

	s, errE = c.getPushRules(client, &configuration)
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

	s, errE = c.getProtectedTags(client, &configuration)
	if errE != nil {
		return errE
	}
	hasSensitive = hasSensitive || s

	s, errE = c.getVariables(client, &configuration)
	if errE != nil {
		return errE
	}
	hasSensitive = hasSensitive || s

	s, errE = c.getPipelineSchedules(client, &configuration)
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
		errE := errors.WithMessage(err, "cannot write configuration")
		errors.Details(errE)["path"] = c.Output
		return errE
	}

	fmt.Fprintf(os.Stderr, "Got everything.\n")
	if hasSensitive {
		args := []string{os.Args[0]}
		if globals.ChangeTo != "" {
			args = append(args, "-C", string(globals.ChangeTo))
		}
		args = append(args, "sops", "--encrypt", "--mac-only-encrypted", "--in-place")
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
