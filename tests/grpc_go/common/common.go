// Package common: demo.Greeter's messages by hand (a string field 1, an int64 field 1), so the Go side of
// tests/grpc_interop.sh needs no protoc: grpc-go carries raw bytes through a codec named "proto".
package common

import (
	"encoding/binary"
	"fmt"

	"google.golang.org/grpc/encoding"
)

type RawCodec struct{}

func (RawCodec) Marshal(v any) ([]byte, error) {
	b, ok := v.([]byte)
	if !ok {
		return nil, fmt.Errorf("RawCodec: not bytes: %T", v)
	}
	return b, nil
}

func (RawCodec) Unmarshal(data []byte, v any) error {
	p, ok := v.(*[]byte)
	if !ok {
		return fmt.Errorf("RawCodec: not *[]byte: %T", v)
	}
	*p = append([]byte(nil), data...)
	return nil
}

func (RawCodec) Name() string { return "proto" }

func init() { encoding.RegisterCodec(RawCodec{}) }

// Str encodes message { string field 1 = s } (HelloRequest, HelloReply).
func Str(s string) []byte {
	out := []byte{0x0a}
	out = binary.AppendUvarint(out, uint64(len(s)))
	return append(out, s...)
}

// GetStr reads field 1 as a string.
func GetStr(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	n, k := binary.Uvarint(b[1:])
	return string(b[1+k : 1+k+int(n)])
}

// Num encodes message { int64 field 1 = n } (Num).
func Num(n int64) []byte {
	if n == 0 {
		return nil
	}
	return binary.AppendUvarint([]byte{0x08}, uint64(n))
}

func GetNum(b []byte) int64 {
	if len(b) == 0 {
		return 0
	}
	v, _ := binary.Uvarint(b[1:])
	return int64(v)
}
