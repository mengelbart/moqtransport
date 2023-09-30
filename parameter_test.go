package moqtransport

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.lrz.de/cm/moqtransport/varint"
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
		buf     []byte
		expectN int
		expect  parameter
		err     error
	}{
		{
			buf:     []byte{byte(roleParameterKey), 0x01, byte(ingestionRole)},
			expectN: 3,
			expect:  roleParameter(ingestionRole),
			err:     nil,
		},
		{
			buf:     append(append([]byte{byte(pathParameterKey)}, varint.Append([]byte{}, uint64(len("/path/param")))...), "/path/param"...),
			expectN: 13,
			expect:  pathParameter("/path/param"),
			err:     nil,
		},
		{
			buf:     []byte{},
			expectN: 0,
			expect:  nil,
			err:     io.EOF,
		},
		{
			buf:     []byte{0x05},
			expectN: 0,
			expect:  nil,
			err:     errUnknownParameter,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			r := bytes.NewReader(tc.buf)
			res, n, err := parseParameter(r)
			if tc.err != nil {
				assert.Equal(t, tc.err, err)
				assert.Equal(t, tc.expect, res)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expectN, n)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParameterLen(t *testing.T) {
	cases := []struct {
		p      parameter
		expect uint64
	}{
		{
			p:      pathParameter(""),
			expect: 0,
		},
		{
			p:      pathParameter("HELLO"),
			expect: 5,
		},
		{
			p:      roleParameter(ingestionRole),
			expect: 1,
		},
		{
			p:      roleParameter(derliveryRole),
			expect: 1,
		},
		{
			p:      roleParameter(ingestionDeliveryRole),
			expect: 1,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.p.length()
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseParameters(t *testing.T) {
	cases := []struct {
		r      messageReader
		len    int
		expect parameters
		err    error
	}{
		{
			r:      bytes.NewReader(nil),
			len:    0,
			expect: parameters{},
			err:    nil,
		},
		{
			r:      bytes.NewReader([]byte{}),
			len:    0,
			expect: parameters{},
			err:    nil,
		},
		{
			r:      bytes.NewReader([]byte{0x01, 0x01, 'A'}),
			len:    3,
			expect: parameters{pathParameterKey: pathParameter("A")},
			err:    nil,
		},
		{
			r:   bytes.NewReader([]byte{0x00, 0x01, 0x01, 0x01, 0x01, 'A'}),
			len: 6,
			expect: parameters{
				roleParameterKey: roleParameter(ingestionRole),
				pathParameterKey: pathParameter("A"),
			},
			err: nil,
		},
		{
			r:      bytes.NewReader([]byte{0x01, 0x01, 'A', 0x02, 0x02, 0x02, 0x02}),
			len:    3,
			expect: parameters{pathParameterKey: pathParameter("A")},
			err:    nil,
		},
		{
			r:      bytes.NewReader([]byte{}),
			len:    2,
			expect: nil,
			err:    io.EOF,
		},
		{
			r:      bytes.NewReader([]byte{0x01, 0x01, 0x01, 0x02, 0x02, 0x02, 0x02}),
			len:    5,
			expect: nil,
			err:    errUnknownParameter,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res, err := parseParameters(tc.r, tc.len)
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
