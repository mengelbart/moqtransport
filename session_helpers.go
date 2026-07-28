package moqtransport

import (
	"slices"

	"github.com/mengelbart/moqtransport/internal/wire"
)

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

func validateAuthParameter(subscribeParameters wire.KVPList) (string, error) {
	index := slices.IndexFunc(subscribeParameters, func(p wire.KeyValuePair) bool {
		return p.Type == wire.AuthorizationTokenParameterKey
	})
	if index < 0 {
		return "", nil
	}
	return string(subscribeParameters[index].ValueBytes), nil
}
