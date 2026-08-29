package moqtransport

import (
	"testing"

	"github.com/mengelbart/moqtransport/internal/wire"
	"github.com/stretchr/testify/assert"
)

func TestValidatePathParameter(t *testing.T) {
	path := wire.KeyValuePair{Type: wire.PathParameterKey, Bytes: []byte("/path")}
	other := wire.KeyValuePair{Type: wire.MaxRequestIDParameterKey, Varint: 1}

	cases := []struct {
		name           string
		parameters     []wire.KeyValuePair
		protocolIsQUIC bool
		expect         string
		err            error
	}{
		{
			name:           "missing on QUIC",
			protocolIsQUIC: true,
			err:            errMissingPathParameter,
		},
		{
			name:           "missing on WebTransport",
			protocolIsQUIC: false,
		},
		{
			name:           "first parameter on QUIC",
			parameters:     []wire.KeyValuePair{path, other},
			protocolIsQUIC: true,
			expect:         "/path",
		},
		{
			name:           "later parameter on QUIC",
			parameters:     []wire.KeyValuePair{other, path},
			protocolIsQUIC: true,
			expect:         "/path",
		},
		{
			name:           "first parameter on WebTransport",
			parameters:     []wire.KeyValuePair{path, other},
			protocolIsQUIC: false,
			err:            errUnexpectedPathParameter,
		},
		{
			name:           "later parameter on WebTransport",
			parameters:     []wire.KeyValuePair{other, path},
			protocolIsQUIC: false,
			err:            errUnexpectedPathParameter,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := validatePathParameter(tc.parameters, tc.protocolIsQUIC)
			assert.Equal(t, tc.expect, res)
			assert.Equal(t, tc.err, err)
		})
	}
}
