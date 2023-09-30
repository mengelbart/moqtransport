package moqtransport

import "gitlab.lrz.de/cm/moqtransport/varint"

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
