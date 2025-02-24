package wire

import (
	"fmt"

	"github.com/quic-go/quic-go/quicvarint"
)

type Version uint64

const (
	Draft_ietf_moq_transport_00 Version = 0xff000000
	Draft_ietf_moq_transport_01 Version = 0xff000001
	Draft_ietf_moq_transport_02 Version = 0xff000002
	Draft_ietf_moq_transport_03 Version = 0xff000003
	Draft_ietf_moq_transport_04 Version = 0xff000004
	Draft_ietf_moq_transport_05 Version = 0xff000005
	Draft_ietf_moq_transport_06 Version = 0xff000006
	Draft_ietf_moq_transport_07 Version = 0xff000007
	Draft_ietf_moq_transport_08 Version = 0xff000008

	CurrentVersion = Draft_ietf_moq_transport_08
)

var SupportedVersions = []Version{CurrentVersion}

func (v Version) String() string {
	return fmt.Sprintf("0x%x", uint64(v))
}

func (v Version) Len() uint64 {
	return uint64(quicvarint.Len(uint64(v)))
}

type versions []Version

func (v versions) String() string {
	res := "["
	for i, e := range v {
		if i < len(v)-1 {
			res += fmt.Sprintf("%v, ", e)
		} else {
			res += fmt.Sprintf("%v", e)
		}
	}
	res += "]"
	return res
}

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

func (vs *versions) parse(data []byte) (int, error) {
	numVersions, parsed, err := quicvarint.Parse(data)
	if err != nil {
		return parsed, err
	}
	data = data[parsed:]

	for i := 0; i < int(numVersions); i++ {
		v, n, err := quicvarint.Parse(data)
		parsed += n
		if err != nil {
			return parsed, err
		}
		data = data[n:]
		*vs = append(*vs, Version(v))
	}
	return parsed, nil
}
