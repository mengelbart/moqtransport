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
