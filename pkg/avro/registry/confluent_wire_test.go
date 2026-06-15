package registry

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncodeDecodeConfluent_roundTrip(t *testing.T) {
	tests := []struct {
		name          string
		id            int32
		datum         []byte
		expectedID    int32
		expectedDatum []byte
		wantErr       bool
	}{
		{
			name:          "round trip encodes schema id and datum",
			id:            42,
			datum:         []byte{0x02, 0xaa, 0xbb},
			expectedID:    42,
			expectedDatum: []byte{0x02, 0xaa, 0xbb},
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wire := EncodeConfluent(tt.id, tt.datum)
			gotID, gotDatum, err := DecodeConfluent(wire)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedID, gotID)
			assert.Equal(t, tt.expectedDatum, gotDatum)
		})
	}
}

func TestDecodeConfluent_rejectsShort(t *testing.T) {
	tests := []struct {
		name    string
		wire    []byte
		wantErr bool
	}{
		{
			name:    "rejects short wire payload",
			wire:    []byte{1, 2, 3},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := DecodeConfluent(tt.wire)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDecodeConfluent_rejectsBadMagic(t *testing.T) {
	tests := []struct {
		name    string
		wire    []byte
		wantErr bool
	}{
		{
			name:    "rejects bad magic byte",
			wire:    []byte{9, 0, 0, 0, 1},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := DecodeConfluent(tt.wire)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
