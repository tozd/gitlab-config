package config

import (
	"io"
	"os"

	"github.com/alecthomas/kong"
	"gitlab.com/tozd/go/errors"
	yaml "gopkg.in/yaml.v3"
)

// We do not use type=existingfile for Input or type=path for Output because we want relative paths.
type FormatCommand struct {
	Input  string `short:"i" placeholder:"PATH" type:"string" required:"" help:"Where to load the configuration from. Can be \"-\" for stdin."`
	Output string `short:"o" placeholder:"PATH" type:"string" default:"-" help:"Where to save the configuration to. Can be the same as input to overwrite it. Default is \"${default}\"."`
}

func (c *FormatCommand) Run(globals *Globals) errors.E {
	if globals.ChangeTo != "" {
		err := os.Chdir(globals.ChangeTo)
		if err != nil {
			return errors.Wrapf(err, `cannot change current working directory to "%s"`, globals.ChangeTo)
		}
	}

	var input []byte
	var err error
	if c.Input != "-" {
		input, err = os.ReadFile(kong.ExpandPath(c.Input))
	} else {
		input, err = io.ReadAll(os.Stdin)
	}
	if err != nil {
		return errors.Wrapf(err, `cannot read configuration from "%s"`, c.Input)
	}

	var node yaml.Node
	err = yaml.Unmarshal(input, &node)
	if err != nil {
		return errors.Wrapf(err, `cannot unmarshal configuration from "%s"`, c.Input)
	}

	descriptions, errE := getProjectConfigDescriptions()
	if errE != nil {
		return errE
	}

	return writeYAML(&node, descriptions, c.Output)
}
