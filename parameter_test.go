package moqtransport

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/mengelbart/moqtransport/varint"

	"github.com/stretchr/testify/assert"
)

func TestParameterAppend(t *testing.T) {
	cases := []struct {
		p      parameter
		buf    []byte
		expect []byte
	}{
		{
			p:      pathParameter(""),
			buf:    nil,
			expect: []byte{0x01, 0x00},
		},
		{
			p:      pathParameter("A"),
			buf:    nil,
			expect: []byte{0x01, 0x01, 'A'},
		},
		{
			p:      pathParameter("A"),
			buf:    []byte{},
			expect: []byte{0x01, 0x01, 'A'},
		},
		{
			p:      pathParameter("A"),
			buf:    []byte{0x01, 0x02},
			expect: []byte{0x01, 0x02, 0x01, 0x01, 'A'},
		},
		{
			p:      roleParameter(ingestionRole),
			buf:    nil,
			expect: []byte{0x00, 0x01, 0x01},
		},
		{
			p:      roleParameter(derliveryRole),
			buf:    nil,
			expect: []byte{0x00, 0x01, 0x02},
		},
		{
			p:      roleParameter(ingestionDeliveryRole),
			buf:    []byte{},
			expect: []byte{0x00, 0x01, 0x03},
		},
		{
			p:      roleParameter(ingestionDeliveryRole),
			buf:    []byte{0x01, 0x02},
			expect: []byte{0x01, 0x02, 0x00, 0x01, 0x03},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.p.append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseParameter(t *testing.T) {
	cases := []struct {
		buf    []byte
		expect parameter
		err    error
	}{
		{
			buf:    []byte{byte(roleParameterKey), 0x01, byte(ingestionRole)},
			expect: roleParameter(ingestionRole),
			err:    nil,
		},
		{
			buf:    append(append([]byte{byte(pathParameterKey)}, varint.Append([]byte{}, uint64(len("/path/param")))...), "/path/param"...),
			expect: pathParameter("/path/param"),
			err:    nil,
		},
		{
			buf:    []byte{},
			expect: nil,
			err:    io.EOF,
		},
		{
			buf:    []byte{0x05},
			expect: nil,
			err:    errUnknownParameter,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			r := bytes.NewReader(tc.buf)
			res, err := parseParameter(r)
			if tc.err != nil {
				assert.Equal(t, tc.err, err)
				assert.Equal(t, tc.expect, res)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseParameters(t *testing.T) {
	cases := []struct {
		r      messageReader
		expect parameters
		err    error
	}{
		{
			r:      nil,
			expect: nil,
			err:    errInvalidMessageReader,
		},
		{
			r:      bytes.NewReader(nil),
			expect: nil,
			err:    io.EOF,
		},
		{
			r:      bytes.NewReader([]byte{}),
			expect: nil,
			err:    io.EOF,
		},
		{
			r:      bytes.NewReader([]byte{0x01, 0x01, 0x01, 'A'}),
			expect: parameters{pathParameterKey: pathParameter("A")},
			err:    nil,
		},
		{
			r: bytes.NewReader([]byte{0x02, 0x00, 0x01, 0x01, 0x01, 0x01, 'A'}),
			expect: parameters{
				roleParameterKey: roleParameter(ingestionRole),
				pathParameterKey: pathParameter("A"),
			},
			err: nil,
		},
		{
			r:      bytes.NewReader([]byte{0x01, 0x01, 0x01, 'A', 0x02, 0x02, 0x02, 0x02}),
			expect: parameters{pathParameterKey: pathParameter("A")},
			err:    nil,
		},
		{
			r:      bytes.NewReader([]byte{}),
			expect: nil,
			err:    io.EOF,
		},
		{
			r:      bytes.NewReader([]byte{0x02, 0x01, 0x01, 0x01, 0x02, 0x02, 0x02, 0x02}),
			expect: nil,
			err:    errUnknownParameter,
		},
		{
			r:      bytes.NewReader([]byte{0x02, 0x00, 0x01, 0x01, 0x00, 0x01, 'A'}),
			expect: nil,
			err:    errDuplicateParameter,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res, err := parseParameters(tc.r)
			if tc.err != nil {
				assert.Equal(t, tc.err, err)
				assert.Equal(t, tc.expect, res)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expect, res)
		})
	}
}
