package moqtransport

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionsLen(t *testing.T) {
	cases := []struct {
		versions versions
		expected uint64
	}{
		{
			versions: []version{},
			expected: 0,
		},
		{
			versions: []version{version(0)},
			expected: 1,
		},
		{
			versions: []version{version(DRAFT_IETF_MOQ_TRANSPORT_00)},
			expected: 1,
		},
		{
			versions: []version{version(1024)},
			expected: 2,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.versions.Len()
			assert.Equal(t, tc.expected, res)
		})
	}
}

func TestVersionsAppend(t *testing.T) {
	cases := []struct {
		versions versions
		buf      []byte
		expected []byte
	}{
		{
			versions: []version{},
			buf:      []byte{},
			expected: []byte{0x00},
		},
		{
			versions: []version{DRAFT_IETF_MOQ_TRANSPORT_00},
			buf:      []byte{},
			expected: []byte{0x01, DRAFT_IETF_MOQ_TRANSPORT_00},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.versions.append(tc.buf)
			assert.Equal(t, tc.expected, res)
		})
	}
}

func TestParseVersions(t *testing.T) {
	cases := []struct {
		r      messageReader
		expect versions
		err    error
	}{
		{
			r:      nil,
			expect: nil,
			err:    errInvalidMessageReader,
		},
		{
			r:      bytes.NewReader([]byte{0x01, DRAFT_IETF_MOQ_TRANSPORT_00}),
			expect: []version{DRAFT_IETF_MOQ_TRANSPORT_00},
			err:    nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res, err := parseVersions(tc.r)
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
