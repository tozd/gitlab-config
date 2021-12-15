package config

import (
	"fmt"
	"os"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/stretchr/testify/require"
)

func TestE2E(t *testing.T) {
	if os.Getenv("GITLAB_API_TOKEN") == "" {
		t.Skip("GITLAB_API_TOKEN is not available")
	}

	tempDir := t.TempDir()
	projectID, errE := inferProjectID(".")
	require.NoError(t, errE)

	for _, cmd := range []string{"get", "set"} {
		t.Run(fmt.Sprintf("case=%s", cmd), func(t *testing.T) {
			var commands Commands
			parser, err := kong.New(&commands, kong.Exit(func(code int) {
				if code != 0 {
					t.Errorf("Kong exited with code %d", code)
				}
			}))
			require.NoError(t, err)

			ctx, err := parser.Parse([]string{"-C", tempDir, cmd, "-p", projectID})
			require.NoError(t, err)

			err = ctx.Run(&commands.Globals)
			require.NoError(t, err)
		})
	}
}
