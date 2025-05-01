package wire

import (
	"fmt"

	"github.com/quic-go/quic-go/quicvarint"
)

const (
	PathParameterKey                  = 0x01
	MaxRequestIDParameterKey          = 0x02
	MaxAuthTokenCacheSizeParameterKey = 0x03
)

const (
	AuthorizationTokenParameterKey = 0x01
	DeliveryTimeoutParameterKey    = 0x02
	MaxCacheDurationParameterKey   = 0x03
)

type Parameters []KeyValuePair

func (pp Parameters) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(len(pp)))
	for _, p := range pp {
		buf = p.append(buf)
	}
	return buf
}

func (pp Parameters) String() string {
	res := "["
	i := 0
	for _, v := range pp {
		if i < len(pp)-1 {
			res += fmt.Sprintf("%v, ", v)
		} else {
			res += fmt.Sprintf("%v", v)
		}
		i++
	}
	return res + "]"
}

func (pp *Parameters) parse(data []byte) error {
	numParameters, n, err := quicvarint.Parse(data)
	if err != nil {
		return err
	}
	data = data[n:]

	for i := uint64(0); i < numParameters; i++ {
		param := KeyValuePair{}
		n, err := param.parse(data)
		if err != nil {
			return err
		}
		data = data[n:]
		*pp = append(*pp, param)
	}
	return nil
}
