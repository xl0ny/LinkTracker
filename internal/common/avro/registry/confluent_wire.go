package registry

import (
	"encoding/binary"
	"errors"
	"fmt"
)

const magicByte byte = 0

// minConfluentWireLen is 1-byte magic + 4-byte schema id.
const minConfluentWireLen = 5

func EncodeConfluent(schemaID int32, avroDatum []byte) []byte {
	out := make([]byte, 1+4+len(avroDatum))
	out[0] = magicByte
	binary.BigEndian.PutUint32(out[1:], uint32(schemaID))
	copy(out[5:], avroDatum)
	return out
}

func DecodeConfluent(wire []byte) (schemaID int32, datum []byte, err error) {
	if len(wire) < minConfluentWireLen {
		return 0, nil, errors.New("confluent wire: payload too short")
	}
	if wire[0] != magicByte {
		return 0, nil, fmt.Errorf("confluent wire: bad magic byte %d", wire[0])
	}
	id := int32(binary.BigEndian.Uint32(wire[1:5]))
	return id, wire[5:], nil
}
