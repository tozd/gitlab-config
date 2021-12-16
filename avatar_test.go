package config

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckAvatarExtension(t *testing.T) {
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
		t.Run(fmt.Sprintf("case=%s", tt.ext), func(t *testing.T) {
			err := checkAvatarExtension(tt.ext)
			if tt.want {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
