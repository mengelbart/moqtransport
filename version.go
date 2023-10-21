package moqtransport

import (
	"github.com/mengelbart/moqtransport/varint"
)

type version uint64

func (v version) Len() uint64 {
	return uint64(varint.Len(uint64(v)))
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
	buf = varint.Append(buf, uint64(len(v)))
	for _, vv := range v {
		buf = varint.Append(buf, uint64(vv))
	}
	return buf
}

func parseVersions(r messageReader) (versions, error) {
	if r == nil {
		return nil, errInvalidMessageReader
	}
	numSupportedVersions, err := varint.Read(r)
	if err != nil {
		return nil, err
	}
	var vs versions
	for i := 0; i < int(numSupportedVersions); i++ {
		var v uint64
		v, err = varint.Read(r)
		if err != nil {
			return nil, err
		}
		vs = append(vs, version(v))
	}
	return vs, nil
}
