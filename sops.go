package config

import (
	"github.com/tozd/sops/v3/cmd/mainimpl"
	"gitlab.com/tozd/go/errors"
)

// SopsCommand describes parameters for the sops command.
type SopsCommand struct {
	Args []string `arg:"" help:"Arguments passed on to SOPS." name:"arg" optional:""`
}

// Run runs the sops command.
func (c *SopsCommand) Run(_ *Globals) errors.E {
	args := append([]string{"sops"}, c.Args...)
	mainimpl.Main(args)

	return nil
}
