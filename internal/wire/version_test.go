package wire

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionsLen(t *testing.T) {
	cases := []struct {
		versions versions
		expected uint64
	}{
		{
			versions: []Version{},
			expected: 0,
		},
		{
			versions: []Version{Version(0)},
			expected: 1,
		},
		{
			versions: []Version{Version(CurrentVersion)},
			expected: 8,
		},
		{
			versions: []Version{Version(1024)},
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
			versions: []Version{},
			buf:      []byte{},
			expected: []byte{0x00},
		},
		{
			versions: []Version{0},
			buf:      []byte{},
			expected: []byte{0x01, 0x00},
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
		data   []byte
		expect versions
		err    error
	}{
		{
			data:   nil,
			expect: versions{},
			err:    io.EOF,
		},
		{
			data:   []byte{0x01, 0x00},
			expect: versions{0},
			err:    nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			reader := bufio.NewReader(bytes.NewReader(tc.data))
			res := versions{}
			err := res.parse(reader)
			assert.Equal(t, tc.expect, res)
			if tc.err != nil {
				assert.Equal(t, tc.err, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
