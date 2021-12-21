package config

import (
	"os"

	"github.com/tozd/sops/v3/cmd/mainimpl"
	"gitlab.com/tozd/go/errors"
)

// SopsCommand describes parameters for the sops command.
type SopsCommand struct {
	Arg []string `arg:"" optional:"" passthrough:"" help:"Arguments passed on to SOPS."`
}

// Run runs the sops command.
func (c *SopsCommand) Run(globals *Globals) errors.E {
	if globals.ChangeTo != "" {
		err := os.Chdir(globals.ChangeTo)
		if err != nil {
			return errors.Wrapf(err, `cannot change current working directory to "%s"`, globals.ChangeTo)
		}
	}

	args := append([]string{"sops"}, c.Arg...)
	mainimpl.Main(args)

	return nil
}
