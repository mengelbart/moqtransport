package moqtransport

import (
	"fmt"

	"github.com/quic-go/quic-go/quicvarint"
)

type varintParameter struct {
	K uint64
	V uint64
}

func (r varintParameter) String() string {
	return fmt.Sprintf("key: %v, value: %v", r.K, r.V)
}

func (r varintParameter) key() uint64 {
	return r.K
}

func (r varintParameter) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, r.K)
	buf = quicvarint.Append(buf, uint64(quicvarint.Len(r.V)))
	buf = quicvarint.Append(buf, r.V)
	return buf
}

// TODO: Compare varint length with parameter length and skip if it doesn't
// match.
func parseVarintParameter(r messageReader, key uint64) (varintParameter, error) {
	_, err := quicvarint.Read(r)
	if err != nil {
		return varintParameter{}, err
	}
	v, err := quicvarint.Read(r)
	if err != nil {
		return varintParameter{}, err
	}
	return varintParameter{
		K: key,
		V: v,
	}, nil
}
