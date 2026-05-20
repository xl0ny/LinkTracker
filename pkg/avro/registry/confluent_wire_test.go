package registry

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEncodeDecodeConfluent_roundTrip(t *testing.T) {
	id := int32(42)
	datum := []byte{0x02, 0xaa, 0xbb}
	wire := EncodeConfluent(id, datum)
	gotID, gotDatum, err := DecodeConfluent(wire)
	require.NoError(t, err)
	require.Equal(t, id, gotID)
	require.Equal(t, datum, gotDatum)
}

func TestDecodeConfluent_rejectsShort(t *testing.T) {
	_, _, err := DecodeConfluent([]byte{1, 2, 3})
	require.Error(t, err)
}

func TestDecodeConfluent_rejectsBadMagic(t *testing.T) {
	_, _, err := DecodeConfluent([]byte{9, 0, 0, 0, 1})
	require.Error(t, err)
}
