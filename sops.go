package config

import (
	"github.com/tozd/sops/v3/cmd/mainimpl"
	"gitlab.com/tozd/go/errors"
)

// SopsCommand describes parameters for the sops command.
type SopsCommand struct {
	Arg []string `arg:"" optional:"" help:"Arguments passed on to SOPS."`
}

// Run runs the sops command.
func (c *SopsCommand) Run(globals *Globals) errors.E {
	args := append([]string{"sops"}, c.Arg...)
	mainimpl.Main(args)

	return nil
}
