package moqtransport

import "gitlab.lrz.de/cm/moqtransport/varint"

type Version uint64

func (v Version) Len() uint64 {
	return uint64(varint.Len(uint64(v)))
}

type Versions []Version

func (v Versions) Len() uint64 {
	l := uint64(0)
	for _, x := range v {
		l = l + x.Len()
	}
	return l
}
