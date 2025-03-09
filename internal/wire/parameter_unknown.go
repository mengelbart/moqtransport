package wire

import (
	"encoding/hex"
	"fmt"
	"log/slog"

	"github.com/mengelbart/qlog"
	"github.com/quic-go/quic-go/quicvarint"
)

type UnknownParameter struct {
	Type  uint64
	Value []byte
}

func (r UnknownParameter) String() string {
	return fmt.Sprintf("{key: %v, value: '%v'}", r.Type, hex.EncodeToString(r.Value))
}

func (r UnknownParameter) key() uint64 {
	return r.Type
}

func (r UnknownParameter) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, r.Type)
	return append(buf, r.Value...)
}

func (r UnknownParameter) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("name", "unknown"),
		slog.Uint64("name_bytes", r.Type),
		slog.Uint64("length", uint64(len(r.Value))),
		slog.Any("value_bytes", qlog.RawInfo{
			Length:        uint64(len(r.Value)),
			PayloadLength: uint64(len(r.Value)),
			Data:          r.Value,
		}.LogValue()),
	)
}

func parseUnknownParameter(data []byte, typ uint64, _ string) (Parameter, int, error) {
	parsed := 0
	l, n, err := quicvarint.Parse(data)
	if err != nil {
		return UnknownParameter{}, n, err
	}
	parsed += n
	data = data[n:]

	return UnknownParameter{
		Type:  typ,
		Value: data[:l],
	}, parsed + int(l), nil
}
