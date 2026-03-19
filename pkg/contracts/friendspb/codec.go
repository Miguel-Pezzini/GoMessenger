package friendspb

import (
	"encoding/json"

	"google.golang.org/grpc/encoding"
)

type jsonCodec struct{}

func (jsonCodec) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func (jsonCodec) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func (jsonCodec) Name() string {
	return "json"
}

var codecInstance encoding.Codec = jsonCodec{}

func init() {
	encoding.RegisterCodec(codecInstance)
}

func JSONCodec() encoding.Codec {
	return codecInstance
}
