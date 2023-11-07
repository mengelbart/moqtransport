package moqtransport

import (
	"fmt"

	"github.com/quic-go/quic-go/quicvarint"
)

type version uint64

func (v version) String() string {
	return fmt.Sprintf("0x%x", uint64(v))
}

func (v version) Len() uint64 {
	return uint64(quicvarint.Len(uint64(v)))
}

type versions []version

func (v versions) Len() uint64 {
	l := uint64(0)
	for _, x := range v {
		l = l + x.Len()
	}
	return l
}

func (v versions) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(len(v)))
	for _, vv := range v {
		buf = quicvarint.Append(buf, uint64(vv))
	}
	return buf
}

func parseVersions(r messageReader) (versions, error) {
	if r == nil {
		return nil, errInvalidMessageReader
	}
	numSupportedVersions, err := quicvarint.Read(r)
	if err != nil {
		return nil, err
	}
	var vs versions
	for i := 0; i < int(numSupportedVersions); i++ {
		var v uint64
		v, err = quicvarint.Read(r)
		if err != nil {
			return nil, err
		}
		vs = append(vs, version(v))
	}
	return vs, nil
}
