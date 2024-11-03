package wire

import (
	"fmt"
	"io"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockReader struct {
	reads [][]byte
	index int
}

func (r *mockReader) Read(p []byte) (int, error) {
	if r.index == len(r.reads) {
		log.Printf("mock reader EOF")
		return 0, io.EOF
	}
	log.Printf("mock reader reading: %v", r.reads[r.index])
	n := copy(p, r.reads[r.index])
	r.index += 1
	return n, nil
}

func TestControlMessageParser(t *testing.T) {
	cases := []struct {
		mr     *mockReader
		expect Message
		err    error
	}{
		{
			mr: &mockReader{
				reads: [][]byte{
					{0x40, byte(messageTypeClientSetup), 0x04, 0x02, 0x00, 0x01, 0x00},
				},
				index: 0,
			},
			expect: &ClientSetupMessage{
				SupportedVersions: []Version{0x00, 0x01},
				SetupParameters:   map[uint64]Parameter{},
			},
			err: nil,
		},
		{
			mr: &mockReader{
				reads: [][]byte{
					{0x40, byte(messageTypeClientSetup), 0x04},
					{0x02, 0x00, 0x01, 0x00},
				},
				index: 0,
			},
			expect: &ClientSetupMessage{
				SupportedVersions: []Version{0x00, 0x01},
				SetupParameters:   map[uint64]Parameter{},
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			p := NewControlMessageParser(tc.mr)
			m, err := p.Parse()
			assert.Equal(t, tc.expect, m)
			if tc.err != nil {
				assert.Equal(t, tc.err, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
