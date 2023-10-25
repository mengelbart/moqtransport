package moqtransport

import "github.com/quic-go/quic-go/quicvarint"

type stringParameter struct {
	k uint64
	v string
}

func (r stringParameter) key() uint64 {
	return r.k
}

func (r stringParameter) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, r.k)
	buf = appendVarIntString(buf, r.v)
	return buf
}

func parseStringParameter(r messageReader, key uint64) (stringParameter, error) {
	v, err := parseVarIntString(r)
	if err != nil {
		return stringParameter{}, err
	}
	return stringParameter{
		k: key,
		v: v,
	}, nil
}
