package wire

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func parseParameterBytes(buf []byte) ([]Parameter, error) {
	r := &boundedReader{reader: bytes.NewReader(buf), n: int64(len(buf))}
	params, err := parseParameters_v18(r)
	if err != nil {
		return nil, err
	}
	if r.remaining() != 0 {
		return nil, errLengthMismatch
	}
	return params, nil
}

func TestEveryParameterTypeHasAnEncoding(t *testing.T) {
	for _, typ := range []uint64{
		ParameterTypeObjectDeliveryTimeout,
		ParameterTypeAuthorizationToken,
		ParameterTypeRendezvousTimeout,
		ParameterTypeSubgroupDeliveryTimeout,
		ParameterTypeExpires,
		ParameterTypeLargestObject,
		ParameterTypeFillTimeout,
		ParameterTypeForward,
		ParameterTypeSubscriberPriority,
		ParameterTypeSubscriptionFilter,
		ParameterTypeGroupOrder,
		ParameterTypeNewGroupRequest,
		ParameterTypeTrackNamespacePrefix,
	} {
		p := Parameter{Type: typ}
		assert.NotZero(t, p.Encoding(), "type 0x%x", typ)
	}
	assert.Zero(t, (&Parameter{Type: 0x1000}).Encoding())
}

func TestParameterRoundTrip(t *testing.T) {
	cases := []struct {
		name  string
		param Parameter
		want  []byte
	}{
		{
			name:  "uint8 zero",
			param: Parameter{Type: ParameterTypeForward, Uint8: 0},
			want:  []byte{0x01, 0x10, 0x00},
		},
		{
			name:  "uint8 max is one raw byte",
			param: Parameter{Type: ParameterTypeSubscriberPriority, Uint8: 255},
			want:  []byte{0x01, 0x20, 0xff},
		},
		{
			name:  "varint",
			param: Parameter{Type: ParameterTypeExpires, Varint: 300},
			want:  []byte{0x01, 0x08, 0x81, 0x2c},
		},
		{
			name:  "location",
			param: Parameter{Type: ParameterTypeLargestObject, Location: Location{Group: 200, Object: 3}},
			want:  []byte{0x01, 0x09, 0x80, 0xc8, 0x03},
		},
		{
			name:  "bytes empty",
			param: Parameter{Type: ParameterTypeSubscriptionFilter},
			want:  []byte{0x01, 0x21, 0x00},
		},
		{
			name:  "bytes",
			param: Parameter{Type: ParameterTypeAuthorizationToken, Bytes: []byte("tok")},
			want:  []byte{0x01, 0x03, 0x03, 't', 'o', 'k'},
		},
		{
			name:  "namespace empty",
			param: Parameter{Type: ParameterTypeTrackNamespacePrefix, Namespace: [][]byte{}},
			want:  []byte{0x01, 0x34, 0x00},
		},
		{
			name:  "namespace",
			param: Parameter{Type: ParameterTypeTrackNamespacePrefix, Namespace: [][]byte{[]byte("a"), []byte("bc")}},
			want:  []byte{0x01, 0x34, 0x02, 0x01, 'a', 0x02, 'b', 'c'},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf := appendParameters_v18(nil, []Parameter{tc.param})
			assert.Equal(t, tc.want, buf)
			got, err := parseParameterBytes(buf)
			require.NoError(t, err)
			require.Len(t, got, 1)
			assert.Equal(t, tc.param, got[0])
		})
	}
}

func TestParseParametersEmpty(t *testing.T) {
	got, err := parseParameterBytes([]byte{0x00})
	require.NoError(t, err)
	assert.Empty(t, got)
	assert.Equal(t, []byte{0x00}, appendParameters_v18(nil, nil))
}

func TestParseParametersUnknownType(t *testing.T) {
	_, err := parseParameterBytes([]byte{0x01, 0x3f, 0x00})
	assert.ErrorIs(t, err, ErrUnknownParameter)
}

func TestParseParametersTruncated(t *testing.T) {
	buf := appendParameters_v18(nil, params())
	for i := 1; i < len(buf); i++ {
		_, err := parseParameterBytes(buf[:i])
		assert.Error(t, err, "truncated to %v of %v bytes", i, len(buf))
		if err != errLengthMismatch {
			assert.ErrorIs(t, err, io.ErrUnexpectedEOF, "truncated to %v of %v bytes", i, len(buf))
		}
	}
}

func TestValidateParameters(t *testing.T) {
	cases := []struct {
		name   string
		scope  ParameterScope
		params []Parameter
		err    error
	}{
		{name: "empty", scope: ScopeSubscribe},
		{
			name:  "subscribe parameters",
			scope: ScopeSubscribe,
			params: []Parameter{
				{Type: ParameterTypeSubscriberPriority, Uint8: 10},
				{Type: ParameterTypeGroupOrder, Uint8: GroupOrderDescending},
				{Type: ParameterTypeForward, Uint8: 0},
				{Type: ParameterTypeSubgroupDeliveryTimeout, Varint: 100},
				{Type: ParameterTypeObjectDeliveryTimeout, Varint: 50},
				{Type: ParameterTypeRendezvousTimeout, Varint: 1000},
				{Type: ParameterTypeNewGroupRequest, Varint: 0},
				{Type: ParameterTypeSubscriptionFilter, Bytes: []byte{0x02}},
				{Type: ParameterTypeAuthorizationToken, Bytes: []byte("a")},
			},
		},
		{
			name:   "unknown type",
			scope:  ScopeSubscribe,
			params: []Parameter{{Type: 0x1000}},
			err:    ErrUnknownParameter,
		},
		{
			name:   "wrong message",
			scope:  ScopeSubscribe,
			params: []Parameter{{Type: ParameterTypeExpires, Varint: 1}},
			err:    ErrParameterScope,
		},
		{
			name:   "fill timeout in fetch",
			scope:  ScopeFetch,
			params: []Parameter{{Type: ParameterTypeFillTimeout, Varint: 1}},
		},
		{
			name:  "duplicate",
			scope: ScopeSubscribe,
			params: []Parameter{
				{Type: ParameterTypeSubscriberPriority, Uint8: 1},
				{Type: ParameterTypeSubscriberPriority, Uint8: 2},
			},
			err: ErrDuplicateParameter,
		},
		{
			name:  "repeated auth token",
			scope: ScopeSubscribe,
			params: []Parameter{
				{Type: ParameterTypeAuthorizationToken, Bytes: []byte("a")},
				{Type: ParameterTypeAuthorizationToken, Bytes: []byte("b")},
			},
		},
		{
			name:   "group order zero",
			scope:  ScopeSubscribe,
			params: []Parameter{{Type: ParameterTypeGroupOrder, Uint8: 0}},
			err:    ErrParameterValue,
		},
		{
			name:   "group order too large",
			scope:  ScopeFetch,
			params: []Parameter{{Type: ParameterTypeGroupOrder, Uint8: 3}},
			err:    ErrParameterValue,
		},
		{
			name:   "forward out of range",
			scope:  ScopeSubscribe,
			params: []Parameter{{Type: ParameterTypeForward, Uint8: 2}},
			err:    ErrParameterValue,
		},
		{
			name:  "request update for subscription",
			scope: ScopeRequestUpdateSubscribe,
			params: []Parameter{
				{Type: ParameterTypeForward, Uint8: 1},
				{Type: ParameterTypeSubscriptionFilter, Bytes: []byte{0x02}},
			},
		},
		{
			name:   "request update for fetch rejects filter",
			scope:  ScopeRequestUpdateFetch,
			params: []Parameter{{Type: ParameterTypeSubscriptionFilter, Bytes: []byte{0x02}}},
			err:    ErrParameterScope,
		},
		{
			name:   "request update for namespace",
			scope:  ScopeRequestUpdateNamespace,
			params: []Parameter{{Type: ParameterTypeTrackNamespacePrefix, Namespace: [][]byte{[]byte("a")}}},
		},
		{
			name:   "request update for other requests only allows auth",
			scope:  ScopeRequestUpdateOther,
			params: []Parameter{{Type: ParameterTypeForward, Uint8: 1}},
			err:    ErrParameterScope,
		},
		{
			name:  "request update ok",
			scope: ScopeRequestUpdateOk,
			params: []Parameter{
				{Type: ParameterTypeExpires, Varint: 1},
				{Type: ParameterTypeLargestObject, Location: Location{Group: 1, Object: 2}},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateParameters(tc.scope, tc.params)
			if tc.err == nil {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tc.err)
		})
	}
}

func TestParameterValueAccessors(t *testing.T) {
	params := []Parameter{
		{Type: ParameterTypeSubscriberPriority, Uint8: 7},
		{Type: ParameterTypeExpires, Varint: 900},
		{Type: ParameterTypeAuthorizationToken, Bytes: []byte("tok")},
		{Type: ParameterTypeLargestObject, Location: Location{Group: 3, Object: 4}},
	}
	assert.Equal(t, uint8(7), Uint8ParameterValue(params, ParameterTypeSubscriberPriority, DefaultSubscriberPriority))
	assert.Equal(t, DefaultForward, Uint8ParameterValue(params, ParameterTypeForward, DefaultForward))
	assert.Equal(t, uint64(900), VarintParameterValue(params, ParameterTypeExpires, 0))
	assert.Equal(t, uint64(5), VarintParameterValue(params, ParameterTypeRendezvousTimeout, 5))

	b, ok := BytesParameterValue(params, ParameterTypeAuthorizationToken)
	assert.True(t, ok)
	assert.Equal(t, []byte("tok"), b)
	_, ok = BytesParameterValue(params, ParameterTypeSubscriptionFilter)
	assert.False(t, ok)

	l, ok := LocationParameterValue(params, ParameterTypeLargestObject)
	assert.True(t, ok)
	assert.Equal(t, Location{Group: 3, Object: 4}, l)
	_, ok = LocationParameterValue(nil, ParameterTypeLargestObject)
	assert.False(t, ok)
}

func TestAllBytesParameterValues(t *testing.T) {
	params := []Parameter{
		{Type: ParameterTypeAuthorizationToken, Bytes: []byte("a")},
		{Type: ParameterTypeSubscriptionFilter, Bytes: []byte{0x02}},
		{Type: ParameterTypeAuthorizationToken, Bytes: []byte("b")},
	}
	assert.Equal(t, [][]byte{[]byte("a"), []byte("b")}, AllBytesParameterValues(params, ParameterTypeAuthorizationToken))
	assert.Nil(t, AllBytesParameterValues(params, ParameterTypeExpires))
	assert.Nil(t, AllBytesParameterValues(nil, ParameterTypeAuthorizationToken))
}
