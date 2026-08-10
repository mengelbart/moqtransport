package varint

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseVarint(t *testing.T) {
	cases := []struct {
		bytes []byte
		value uint64
		count int
		err   error
	}{
		{[]byte{}, 0, 0, io.EOF},
		{[]byte{0x00}, 0, 1, nil},
		{[]byte{0x01}, 1, 1, nil},
		{[]byte{0x7F}, 127, 1, nil},
		{[]byte{0x25}, 37, 1, nil},
		{[]byte{0x80, 0x25}, 37, 2, nil},
		{[]byte{0x80, 0x00}, 0, 2, nil},
		{[]byte{0xED, 0x7F, 0x3E, 0x7D}, 226_442_877, 4, nil},
		{[]byte{0xFA, 0xA1, 0xA0, 0xE4, 0x03, 0xD8}, 2_893_212_287_960, 6, nil},
		{[]byte{0xFC, 0x89, 0x98, 0xAB, 0xC6, 0x6B, 0xC0}, 151_288_809_941_952, 7, nil},
		{[]byte{0xFE, 0xFA, 0x31, 0x8F, 0xA8, 0xE3, 0xCA, 0x11}, 70_423_237_261_249_041, 8, nil},
		{[]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}, 18_446_744_073_709_551_615, 9, nil},
		{[]byte{0x80}, 0, 0, io.EOF},
		{[]byte{0xC0, 0x01}, 0, 0, io.EOF},
		{[]byte{0xFF, 0xFF, 0xFF}, 0, 0, io.EOF},
		{[]byte{0xFE, 0xFA, 0x31, 0x8F, 0xA8, 0xE3, 0xCA}, 0, 0, io.EOF},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%v", c.bytes), func(t *testing.T) {
			value, bytes, err := Parse(c.bytes)
			assert.Equal(t, c.value, value, "Parse(%v) = %d, want %d", c.bytes, value, c.value)
			assert.Equal(t, c.count, bytes, "Parse(%v) = %d bytes, want %d", c.bytes, bytes, c.count)
			assert.Equal(t, c.err, err, "Parse(%v) = %v, want %v", c.bytes, err, c.err)
		})
	}
}

func TestReadVarint(t *testing.T) {
	cases := []struct {
		bytes []byte
		value uint64
		err   error
	}{
		{[]byte{}, 0, io.EOF},
		{[]byte{0x00}, 0, nil},
		{[]byte{0x01}, 1, nil},
		{[]byte{0x7F}, 127, nil},
		{[]byte{0x25}, 37, nil},
		{[]byte{0x80, 0x25}, 37, nil},
		{[]byte{0x80, 0x00}, 0, nil},
		{[]byte{0xED, 0x7F, 0x3E, 0x7D}, 226_442_877, nil},
		{[]byte{0xFA, 0xA1, 0xA0, 0xE4, 0x03, 0xD8}, 2_893_212_287_960, nil},
		{[]byte{0xFC, 0x89, 0x98, 0xAB, 0xC6, 0x6B, 0xC0}, 151_288_809_941_952, nil},
		{[]byte{0xFE, 0xFA, 0x31, 0x8F, 0xA8, 0xE3, 0xCA, 0x11}, 70_423_237_261_249_041, nil},
		{[]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}, 18_446_744_073_709_551_615, nil},
		{[]byte{0x80}, 0, io.EOF},
		{[]byte{0xFF, 0xFF, 0xFF}, 0, io.EOF},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%v", c.bytes), func(t *testing.T) {
			value, err := Read(bytes.NewReader(c.bytes))
			assert.Equal(t, c.value, value, "Read(%v) = %d, want %d", c.bytes, value, c.value)
			assert.Equal(t, c.err, err, "Read(%v) = %v, want %v", c.bytes, err, c.err)
		})
	}
}

// FuzzParse checks that Parse never panics on arbitrary input and stays
// consistent with Read, which sees the same bytes through an io.ByteReader.
func FuzzParse(f *testing.F) {
	for _, seed := range [][]byte{
		{},
		{0x25},
		{0x80, 0x25},
		{0xC0},
		{0xFF, 0xFF, 0xFF},
		{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		value, n, err := Parse(b)
		readValue, readErr := Read(bytes.NewReader(b))
		if err != nil {
			assert.Equal(t, 0, n, "Parse(%v) consumed %d bytes despite failing", b, n)
			assert.Error(t, readErr, "Parse(%v) failed but Read succeeded", b)
			return
		}
		assert.GreaterOrEqual(t, n, 1, "Parse(%v) succeeded without consuming a byte", b)
		assert.LessOrEqual(t, n, len(b), "Parse(%v) consumed more bytes than it was given", b)
		assert.NoError(t, readErr, "Parse(%v) succeeded but Read failed", b)
		assert.Equal(t, value, readValue, "Parse(%v) = %d, but Read = %d", b, value, readValue)
	})
}

func TestAppendVarint(t *testing.T) {
	cases := []struct {
		value uint64
		bytes []byte
	}{
		{0, []byte{0x00}},
		{1, []byte{0x01}},
		{37, []byte{0x25}},
		{127, []byte{0x7F}},
		{226_442_877, []byte{0xED, 0x7F, 0x3E, 0x7D}},
		{2_893_212_287_960, []byte{0xFA, 0xA1, 0xA0, 0xE4, 0x03, 0xD8}},
		{151_288_809_941_952, []byte{0xFC, 0x89, 0x98, 0xAB, 0xC6, 0x6B, 0xC0}},
		{70_423_237_261_249_041, []byte{0xFE, 0xFA, 0x31, 0x8F, 0xA8, 0xE3, 0xCA, 0x11}},
		{18_446_744_073_709_551_615, []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%d", c.value), func(t *testing.T) {
			bytes := Append([]byte{}, c.value)
			assert.Equal(t, c.bytes, bytes, "Append(%d) = %v, want %v", c.value, bytes, c.bytes)
		})
	}
}
