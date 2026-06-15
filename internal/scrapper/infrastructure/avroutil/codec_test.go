package avroutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMustLoadCodecFromFile(t *testing.T) {
	tests := []struct {
		name      string
		schema    string
		wantPanic bool
	}{
		{
			name:      "loads valid schema",
			schema:    `{"type":"record","name":"X","fields":[{"name":"n","type":"int"}]}`,
			wantPanic: false,
		},
		{
			name:      "panics on invalid schema",
			schema:    `{`,
			wantPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "schema.avsc")
			assert.NoError(t, os.WriteFile(path, []byte(tt.schema), 0o600))

			load := func() {
				codec := MustLoadCodecFromFile(path)
				assert.NotNil(t, codec)
			}
			if tt.wantPanic {
				assert.Panics(t, load)
			} else {
				assert.NotPanics(t, load)
			}
		})
	}
}

func TestMustLoadCodecFromFile_missingFile(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{
			name: "panics on missing file",
			path: filepath.Join("missing", "schema.avsc"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Panics(t, func() {
				MustLoadCodecFromFile(tt.path)
			})
		})
	}
}
