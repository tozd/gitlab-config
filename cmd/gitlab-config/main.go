// Command gitlab-config enables keeping GitLab project's config in a local file (e.g., in a git repository).
//
// You can provide some configuration options as environment variables.
package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"

	"gitlab.com/tozd/gitlab/config"
)

const exitCode = 2

// These variables should be set during build time using "-X" ldflags.
var (
	version        = ""
	buildTimestamp = ""
	revision       = ""
)

func main() {
	var commands config.Commands
	ctx := kong.Parse(&commands,
		kong.Description(
			"Enable keeping GitLab project's config in a local file (e.g., in a git repository).\n\n"+
				"You can provide some configuration options as environment variables.",
		),
		kong.Vars{
			"version": fmt.Sprintf("version %s (build on %s, git revision %s)", version, buildTimestamp, revision),
		},
		kong.UsageOnError(),
		kong.Writers(
			os.Stderr,
			os.Stderr,
		),
	)

	err := ctx.Run(&commands.Globals)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "error: %+v", err)
		ctx.Exit(exitCode)
	}
}
