package registry

import (
	"encoding/binary"
	"errors"
	"fmt"
)

const magicByte byte = 0

const (
	confluentMagicByteLen = 1
	confluentSchemaIDLen  = 4
)

const minConfluentWireLen = confluentMagicByteLen + confluentSchemaIDLen

func EncodeConfluent(schemaID int32, avroDatum []byte) []byte {
	out := make([]byte, minConfluentWireLen+len(avroDatum))
	out[0] = magicByte
	binary.BigEndian.PutUint32(out[confluentMagicByteLen:], uint32(schemaID))
	copy(out[minConfluentWireLen:], avroDatum)
	return out
}

func DecodeConfluent(wire []byte) (schemaID int32, datum []byte, err error) {
	if len(wire) < minConfluentWireLen {
		return 0, nil, errors.New("confluent wire: payload too short")
	}
	if wire[0] != magicByte {
		return 0, nil, fmt.Errorf("confluent wire: bad magic byte %d", wire[0])
	}
	id := int32(binary.BigEndian.Uint32(wire[confluentMagicByteLen:minConfluentWireLen]))
	return id, wire[minConfluentWireLen:], nil
}
