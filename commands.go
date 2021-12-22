package config

import (
	"github.com/alecthomas/kong"
)

// GitLab describes parameters needed to connect to GitLab API.
type GitLab struct {
	Project string `short:"p" env:"CI_PROJECT_ID" help:"GitLab project to release to. It can be project ID or <namespace/project_path>. By default it infers it from the repository. Environment variable: ${env}"`
	BaseURL string `short:"B" name:"base" placeholder:"URL" default:"https://gitlab.com" env:"CI_SERVER_URL" help:"Base URL for GitLab API to use. Default is \"${default}\". Environment variable: ${env}"`
	Token   string `short:"t" required:"" env:"GITLAB_API_TOKEN" help:"GitLab API token to use. Environment variable: ${env}"`
	DocsRef string `short:"D" name:"docs" placeholder:"REF" default:"master" env:"DOCS_GIT_REF" help:"Git reference at which to extract API attributes from GitLab's documentation. Default is \"${default}\". Environment variable: ${env}"`
}

// Globals describes top-level (global) flags.
type Globals struct {
	ChangeTo string           `short:"C" placeholder:"PATH" type:"existingdir" env:"CI_PROJECT_DIR" help:"Run as if the program was started in PATH instead of the current working directory. Environment variable: ${env}"`
	Version  kong.VersionFlag `short:"V" help:"Show program's version and exit."`
}

// Commands is used as configuration for Kong command-line parser.
//
// Besides commands themselves it also contains top-level (global) flags.
type Commands struct {
	Globals

	Get  GetCommand  `cmd:"" help:"Save GitLab project's configuration to a local file."`
	Set  SetCommand  `cmd:"" help:"Update GitLab project's configuration based on a local file."`
	Sops SopsCommand `cmd:"" help:"Run SOPS, an editor of encrypted files. See: https://github.com/tozd/sops"`
}
