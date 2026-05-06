package avroutil

import (
	"fmt"
	"os"

	"github.com/linkedin/goavro/v2"
)

func MustLoadCodecFromFile(path string) *goavro.Codec {
	schema, err := os.ReadFile(path)
	if err != nil {
		panic(fmt.Errorf("avroutil: read avro schema %q: %w", path, err))
	}
	codec, err := goavro.NewCodec(string(schema))
	if err != nil {
		panic(fmt.Errorf("avroutil: build avro codec from %q: %w", path, err))
	}
	return codec
}
