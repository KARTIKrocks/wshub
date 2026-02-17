package websocket

import "encoding/json"

// Codec handles message encoding/decoding.
type Codec interface {
	Encode(v any) ([]byte, error)
	Decode(data []byte, v any) error
}

// JSONCodec is the default codec using JSON encoding.
type JSONCodec struct{}

// Encode encodes a value to JSON bytes.
func (c *JSONCodec) Encode(v any) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, ErrEncodingFailed
	}
	return data, nil
}

// Decode decodes JSON bytes into a value.
func (c *JSONCodec) Decode(data []byte, v any) error {
	if err := json.Unmarshal(data, v); err != nil {
		return ErrDecodingFailed
	}
	return nil
}
