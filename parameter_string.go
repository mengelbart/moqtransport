package moqtransport

import (
	"fmt"

	"github.com/quic-go/quic-go/quicvarint"
)

type stringParameter struct {
	K uint64
	V string
}

func (r stringParameter) String() string {
	return fmt.Sprintf("key: %v, value: %v", r.K, r.V)
}

func (r stringParameter) key() uint64 {
	return r.K
}

func (r stringParameter) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, r.K)
	buf = appendVarIntString(buf, r.V)
	return buf
}

func parseStringParameter(r messageReader, key uint64) (stringParameter, error) {
	v, err := parseVarIntString(r)
	if err != nil {
		return stringParameter{}, err
	}
	return stringParameter{
		K: key,
		V: v,
	}, nil
}
