package moqtransport

import "github.com/quic-go/quic-go/quicvarint"

type varintParameter struct {
	k uint64
	v uint64
}

func (r varintParameter) key() uint64 {
	return r.k
}

func (r varintParameter) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, r.k)
	buf = quicvarint.Append(buf, uint64(quicvarint.Len(r.v)))
	buf = quicvarint.Append(buf, r.v)
	return buf
}

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
		k: key,
		v: v,
	}, nil
}
