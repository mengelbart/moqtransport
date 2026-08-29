package moqtransport

import (
	"slices"

	"github.com/mengelbart/moqtransport/internal/wire"
)

//nolint:unused
func validatePathParameter(setupParameters []wire.KeyValuePair, protocolIsQUIC bool) (string, error) {
	index := slices.IndexFunc(setupParameters, func(p wire.KeyValuePair) bool {
		return p.Type == wire.PathParameterKey
	})
	if index < 0 {
		if protocolIsQUIC {
			return "", errMissingPathParameter
		}
		return "", nil
	}
	if !protocolIsQUIC {
		return "", errUnexpectedPathParameter
	}
	return string(setupParameters[index].Bytes), nil
}

//nolint:unused
func validateAuthParameter(subscribeParameters []wire.KeyValuePair) (string, error) {
	index := slices.IndexFunc(subscribeParameters, func(p wire.KeyValuePair) bool {
		return p.Type == wire.AuthorizationTokenParameterKey
	})
	if index < 0 {
		return "", nil
	}
	return string(subscribeParameters[index].Bytes), nil
}
