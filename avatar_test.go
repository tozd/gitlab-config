package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckAvatarExtension(t *testing.T) {
	t.Parallel()

	tests := []struct {
		ext  string
		want bool
	}{
		{".png", false},
		{".jpg", false},
		{".jpeg", false},
		{".gif", false},
		{".ico", false},
		{".mov", true},
		{".", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run("case="+tt.ext, func(t *testing.T) {
			t.Parallel()

			errE := checkAvatarExtension(tt.ext)
			if tt.want {
				assert.Error(t, errE)
			} else {
				assert.NoError(t, errE, "% -+#.1v", errE)
			}
		})
	}
}
