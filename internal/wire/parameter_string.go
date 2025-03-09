package wire

import (
	"fmt"
	"log/slog"

	"github.com/quic-go/quic-go/quicvarint"
)

type StringParameter struct {
	Type  uint64 `json:"-"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (r StringParameter) String() string {
	return fmt.Sprintf("{key: %v, value: '%v'}", r.Type, r.Value)
}

func (r StringParameter) key() uint64 {
	return r.Type
}

func (r StringParameter) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, r.Type)
	buf = appendVarIntBytes(buf, []byte(r.Value))
	return buf
}

func (r StringParameter) LogValue() slog.Value {
	v := slog.GroupValue(
		slog.String("name", r.Name),
		slog.String("value", r.Value),
	)
	slog.Info("string param", "logvalue", v)
	return v
}

func parseStringParameter(data []byte, key uint64, name string) (Parameter, int, error) {
	buf, n, err := parseVarIntBytes(data)
	if err != nil {
		return StringParameter{}, n, err
	}
	p := StringParameter{
		Type:  key,
		Name:  name,
		Value: string(buf),
	}
	return p, n, err
}
