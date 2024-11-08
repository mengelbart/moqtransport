package wire

import (
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAnnounceErrorMessageAppend(t *testing.T) {
	cases := []struct {
		aem    AnnounceErrorMessage
		buf    []byte
		expect []byte
	}{
		{
			aem: AnnounceErrorMessage{
				TrackNamespace: []string{""},
				ErrorCode:      0,
				ReasonPhrase:   "",
			},
			buf: []byte{},
			expect: []byte{
				0x01, 0x00, 0x00, 0x00,
			},
		},
		{
			aem: AnnounceErrorMessage{
				TrackNamespace: []string{"trackname"},
				ErrorCode:      1,
				ReasonPhrase:   "reason",
			},
			buf:    []byte{},
			expect: append(append([]byte{0x01, 0x09}, "trackname"...), append([]byte{0x01, 0x06}, "reason"...)...),
		},
		{
			aem: AnnounceErrorMessage{
				TrackNamespace: []string{"trackname"},
				ErrorCode:      1,
				ReasonPhrase:   "reason",
			},
			buf:    []byte{0x0a, 0x0b, 0x0c, 0x0d},
			expect: append(append([]byte{0x0a, 0x0b, 0x0c, 0x0d, 0x01, 0x09}, "trackname"...), append([]byte{0x01, 0x06}, "reason"...)...),
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.aem.Append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseAnnounceErrorMessage(t *testing.T) {
	cases := []struct {
		data   []byte
		expect *AnnounceErrorMessage
		err    error
	}{
		{
			data:   nil,
			expect: &AnnounceErrorMessage{},
			err:    io.EOF,
		},
		{
			data: []byte{0x01, 0x02, 'n', 's', 0x03},
			expect: &AnnounceErrorMessage{
				TrackNamespace: []string{"ns"},
				ErrorCode:      3,
				ReasonPhrase:   "",
			},
			err: io.EOF,
		},
		{
			data: append(append(append([]byte{0x01, 0x0e}, "tracknamespace"...), 0x01, 0x0d), "reason phrase"...),
			expect: &AnnounceErrorMessage{
				TrackNamespace: []string{"tracknamespace"},
				ErrorCode:      1,
				ReasonPhrase:   "reason phrase",
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := &AnnounceErrorMessage{}
			err := res.parse(tc.data)
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
