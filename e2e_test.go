package config

import (
	"fmt"
	"os"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/stretchr/testify/require"
	"gitlab.com/tozd/go/x"
)

func TestE2E(t *testing.T) {
	t.Parallel()

	if os.Getenv("GITLAB_API_TOKEN") == "" {
		t.Skip("GITLAB_API_TOKEN is not available")
	}

	tempDir := t.TempDir()
	projectID, errE := x.InferGitLabProjectID(".")
	require.NoError(t, errE, "% -+#.1v", errE)

	for _, cmd := range []string{"get", "set"} {
		cmd := cmd

		t.Run(fmt.Sprintf("case=%s", cmd), func(t *testing.T) {
			t.Parallel()

			var commands Commands
			parser, err := kong.New(&commands,
				kong.Vars{
					"defaultDocsRef": DefaultDocsRef,
				},
				kong.Exit(func(code int) {
					if code != 0 {
						t.Errorf("Kong exited with code %d", code)
					}
				}),
			)
			require.NoError(t, err)

			ctx, err := parser.Parse([]string{"-C", tempDir, cmd, "-p", projectID})
			require.NoError(t, err)

			err = ctx.Run(&commands.Globals)
			require.NoError(t, err, "% -+#.1v", err)
		})
	}
}
