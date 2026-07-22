// Package varint implements variable length integers as defined in
// https://datatracker.ietf.org/doc/html/draft-ietf-moq-transport-19#name-variable-length-integers
package varint

import (
	"io"
)

const (
	maxVarint1 = 127
	maxVarint2 = 16383
	maxVarint3 = 2097151
	maxVarint4 = 268435455
	maxVarint5 = 34359738367
	maxVarint6 = 4398046511103
	maxVarint7 = 562949953421311
	maxVarint8 = 72057594037927935
	maxVarint9 = 18446744073709551615
)

func Parse(b []byte) (uint64, int, error) {
	if len(b) == 0 {
		return 0, 0, io.EOF
	}
	// Count leading ones in the first byte
	leadingOnes := 0
	for i := 7; i >= 0; i-- {
		if (b[0] & (1 << uint(i))) == 0 {
			break
		}
		leadingOnes++
	}

	if leadingOnes == 0 {
		return uint64(b[0]), 1, nil
	}

	result := uint64(b[0] & ((1 << uint(7-leadingOnes)) - 1))
	for i := 1; i <= leadingOnes; i++ {
		result = (result << 8) | uint64(b[i])
	}
	return result, 1 + leadingOnes, nil
}

func Append(b []byte, value uint64) []byte {
	if value <= maxVarint1 {
		return append(b, byte(value))
	}
	if value <= maxVarint2 {
		return append(b, byte(0x80|(value>>8)), byte(value))
	}
	if value <= maxVarint3 {
		return append(b, byte(0xC0|(value>>16)), byte(value>>8), byte(value))
	}
	if value <= maxVarint4 {
		return append(b, byte(0xE0|(value>>24)), byte(value>>16), byte(value>>8), byte(value))
	}
	if value <= maxVarint5 {
		return append(b, byte(0xF0|(value>>32)), byte(value>>24), byte(value>>16), byte(value>>8), byte(value))
	}
	if value <= maxVarint6 {
		return append(b, byte(0xF8|(value>>40)), byte(value>>32), byte(value>>24), byte(value>>16), byte(value>>8), byte(value))
	}
	if value <= maxVarint7 {
		return append(b, byte(0xFC|(value>>48)), byte(value>>40), byte(value>>32), byte(value>>24), byte(value>>16), byte(value>>8), byte(value))
	}
	if value <= maxVarint8 {
		return append(b, byte(0xFE|(value>>56)), byte(value>>48), byte(value>>40), byte(value>>32), byte(value>>24), byte(value>>16), byte(value>>8), byte(value))
	}
	return append(b, byte(0xFF), byte(value>>56), byte(value>>48), byte(value>>40), byte(value>>32), byte(value>>24), byte(value>>16), byte(value>>8), byte(value))
}
