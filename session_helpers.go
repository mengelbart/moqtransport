package moqtransport

import (
	"encoding/binary"
	"errors"
	"math"
	"slices"

	"github.com/mengelbart/moqtransport/internal/wire"
	"github.com/quic-go/quic-go/quicvarint"
)

var errControlMessageTooLarge = errors.New("control message too large")

func compileMessage(msg wire.ControlMessage) ([]byte, error) {
	buf := make([]byte, 0, 4096)
	buf = quicvarint.Append(buf, uint64(msg.Type()))
	tl := len(buf)
	buf = append(buf, 0x00, 0x00) // length placeholder
	buf = msg.Append(buf)
	length := len(buf[tl+2:])
	if length > math.MaxUint16 {
		return nil, errControlMessageTooLarge
	}
	binary.BigEndian.PutUint16(buf[tl:tl+2], uint16(length))
	return buf, nil
}

func validatePathParameter(setupParameters wire.KVPList, protocolIsQUIC bool) (string, error) {
	index := slices.IndexFunc(setupParameters, func(p wire.KeyValuePair) bool {
		return p.Type == wire.PathParameterKey
	})
	if index < 0 {
		if protocolIsQUIC {
			return "", errMissingPathParameter
		}
		return "", nil
	}
	if index > 0 && !protocolIsQUIC {
		return "", errUnexpectedPathParameter
	}
	return string(setupParameters[index].ValueBytes), nil
}

func getMaxRequestIDParameter(setupParameters wire.KVPList) uint64 {
	index := slices.IndexFunc(setupParameters, func(p wire.KeyValuePair) bool {
		return p.Type == wire.MaxRequestIDParameterKey
	})
	if index < 0 {
		return 0
	}
	return setupParameters[index].ValueVarInt
}

func validateAuthParameter(subscribeParameters wire.KVPList) (string, error) {
	index := slices.IndexFunc(subscribeParameters, func(p wire.KeyValuePair) bool {
		return p.Type == wire.AuthorizationTokenParameterKey
	})
	if index < 0 {
		return "", nil
	}
	return string(subscribeParameters[index].ValueBytes), nil
}
