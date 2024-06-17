package wire

import (
	"fmt"
	"io"

	"github.com/quic-go/quic-go/quicvarint"
)

type Version uint64

const (
	Draft_ietf_moq_transport_00 Version = 0xff000000
	Draft_ietf_moq_transport_01 Version = 0xff000001
	Draft_ietf_moq_transport_02 Version = 0xff000002
	Draft_ietf_moq_transport_03 Version = 0xff000003
	Draft_ietf_moq_transport_04 Version = 0xff000004

	CurrentVersion = Draft_ietf_moq_transport_04
)

func (v Version) String() string {
	return fmt.Sprintf("0x%x", uint64(v))
}

func (v Version) Len() uint64 {
	return uint64(quicvarint.Len(uint64(v)))
}

type versions []Version

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

func (vs *versions) parse(reader io.ByteReader) error {
	numVersions, err := quicvarint.Read(reader)
	if err != nil {
		return err
	}
	for i := 0; i < int(numVersions); i++ {
		v, err := quicvarint.Read(reader)
		if err != nil {
			return err
		}
		*vs = append(*vs, Version(v))
	}
	return nil
}
