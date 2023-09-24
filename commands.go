package config

import (
	"github.com/alecthomas/kong"
)

const DefaultDocsRef = "v16.4.0-ee"

// GitLab describes parameters needed to connect to GitLab API.
type GitLab struct {
	Project string `env:"CI_PROJECT_ID"          help:"GitLab project to manage config for. It can be project ID or <namespace/project_path>. By default it infers it from the repository. Environment variable: ${env}" short:"p"`
	BaseURL string `default:"https://gitlab.com" env:"CI_SERVER_URL"                                                                                                                                                     help:"Base URL for GitLab API to use. Default is \"${default}\". Environment variable: ${env}"                                                      name:"base" placeholder:"URL" short:"B"`
	Token   string `env:"GITLAB_API_TOKEN"       help:"GitLab API token to use. Environment variable: ${env}"                                                                                                            required:""                                                                                                                                         short:"t"`
	DocsRef string `default:"${defaultDocsRef}"  env:"DOCS_GIT_REF"                                                                                                                                                      help:"Git reference at which to extract API attributes from GitLab's documentation. Default is \"${defaultDocsRef}\". Environment variable: ${env}" name:"docs" placeholder:"REF" short:"D"`
}

// Globals describes top-level (global) flags.
type Globals struct {
	ChangeTo kong.ChangeDirFlag `env:"CI_PROJECT_DIR"                    help:"Run as if the program was started in PATH instead of the current working directory. Environment variable: ${env}" placeholder:"PATH" short:"C"`
	Version  kong.VersionFlag   `help:"Show program's version and exit." short:"V"`
}

// Commands is used as configuration for Kong command-line parser.
//
// Besides commands themselves it also contains top-level (global) flags.
type Commands struct {
	Globals

	Get  GetCommand  `cmd:"" help:"Save GitLab project's configuration to a local file."`
	Set  SetCommand  `cmd:"" help:"Update GitLab project's configuration based on a local file."`
	Sops SopsCommand `cmd:"" help:"Run SOPS, an editor of encrypted files. See: https://github.com/tozd/sops" passthrough:""`
}
