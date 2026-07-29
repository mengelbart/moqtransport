package moqtransport

import (
	"slices"

	"github.com/mengelbart/moqtransport/internal/wire2"
)

func validatePathParameter(setupParameters []wire2.KeyValuePair, protocolIsQUIC bool) (string, error) {
	index := slices.IndexFunc(setupParameters, func(p wire2.KeyValuePair) bool {
		return p.Type == wire2.PathParameterKey
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
	return string(setupParameters[index].Bytes), nil
}

func validateAuthParameter(subscribeParameters []wire2.KeyValuePair) (string, error) {
	index := slices.IndexFunc(subscribeParameters, func(p wire2.KeyValuePair) bool {
		return p.Type == wire2.AuthorizationTokenParameterKey
	})
	if index < 0 {
		return "", nil
	}
	return string(subscribeParameters[index].Bytes), nil
}
