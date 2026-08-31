package wire

import (
	"errors"
	"math"
	"slices"

	"github.com/mengelbart/moqtransport/varint"
)

// maxKeyValuePairValueLength is the largest value a pair may carry.
const maxKeyValuePairValueLength = 1<<16 - 1

var (
	errKeyValuePairTypeOverflow = errors.New("key value pair type exceeds 2^64-1")
	errKeyValuePairValueTooLong = errors.New("key value pair value exceeds 2^16-1 bytes")
)

type KeyValuePair struct {
	Type   uint64
	Bytes  []byte `proto:"tlv_bytes,if=hasBytes"`
	Varint uint64 `proto:"varint,if=!hasBytes"`
}

// hasBytes reports whether the pair carries a byte string rather than a varint.
func (p *KeyValuePair) hasBytes() bool {
	return p.Type%2 == 1
}

func (p *KeyValuePair) validate() error {
	if len(p.Bytes) > maxKeyValuePairValueLength {
		return errKeyValuePairValueTooLong
	}
	return nil
}

func compareKeyValuePairs(a, b KeyValuePair) int {
	if a.Type < b.Type {
		return -1
	}
	if a.Type > b.Type {
		return 1
	}
	return 0
}

// appendKeyValuePairs_v18 writes pairs without any outer framing, encoding each
// type as a delta from the previous one. The pairs are sorted by type first,
// because a delta encoding cannot express a decreasing sequence.
func appendKeyValuePairs_v18(buf []byte, pairs []KeyValuePair) []byte {
	if !slices.IsSortedFunc(pairs, compareKeyValuePairs) {
		pairs = slices.Clone(pairs)
		slices.SortStableFunc(pairs, compareKeyValuePairs)
	}
	prev := uint64(0)
	for i := range pairs {
		buf = varint.Append(buf, pairs[i].Type-prev)
		buf = pairs[i].append_v18(buf)
		prev = pairs[i].Type
	}
	return buf
}

func parseKeyValuePair_v18(r messageReader, prev uint64) (KeyValuePair, error) {
	delta, err := varint.Read(r)
	if err != nil {
		return KeyValuePair{}, err
	}
	if delta > math.MaxUint64-prev {
		return KeyValuePair{}, errKeyValuePairTypeOverflow
	}
	p := KeyValuePair{Type: prev + delta}
	if err := p.parse_v18(r); err != nil {
		return KeyValuePair{}, err
	}
	return p, p.validate()
}

// parseKeyValuePairsCount_v18 reads a count prefixed list of pairs.
func parseKeyValuePairsCount_v18(r messageReader) ([]KeyValuePair, error) {
	count, err := varint.Read(r)
	if err != nil {
		return nil, err
	}
	pairs := make([]KeyValuePair, 0)
	prev := uint64(0)
	for range count {
		p, err := parseKeyValuePair_v18(r, prev)
		if err != nil {
			return nil, err
		}
		prev = p.Type
		pairs = append(pairs, p)
	}
	return pairs, nil
}

// parseKeyValuePairsTLV_v18 reads a byte length prefixed block of pairs.
func parseKeyValuePairsTLV_v18(r messageReader) ([]KeyValuePair, error) {
	br, err := tlvReader(r)
	if err != nil {
		return nil, err
	}
	return parseKeyValuePairsRemaining_v18(br)
}

// parseKeyValuePairsRemaining_v18 reads pairs until the reader is exhausted.
func parseKeyValuePairsRemaining_v18(r messageReader) ([]KeyValuePair, error) {
	if r.remaining() < 0 {
		return nil, errNoMessageLength
	}
	pairs := make([]KeyValuePair, 0)
	prev := uint64(0)
	for r.remaining() > 0 {
		p, err := parseKeyValuePair_v18(r, prev)
		if err != nil {
			return nil, err
		}
		prev = p.Type
		pairs = append(pairs, p)
	}
	return pairs, nil
}
