package moqtransport

import (
	"bufio"
	"bytes"
	"testing"

	"github.com/mengelbart/moqtransport/varint"
)

func TestPeekFirstVarint(t *testing.T) {
	testCases := []struct {
		name   string
		value  uint64
		expect int
	}{
		{name: "one byte", value: 42, expect: 1},
		{name: "two bytes", value: 0x2f00, expect: 2},
		{name: "nine bytes", value: 1 << 56, expect: 9},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			encoded := varint.Append(nil, testCase.value)
			reader := bufio.NewReader(bytes.NewReader(encoded))

			peeked, err := peekFirstVarint(reader)
			if err != nil {
				t.Fatalf("peekFirstVarint() error = %v", err)
			}
			if len(peeked) != testCase.expect {
				t.Fatalf("peekFirstVarint() length = %d, want %d", len(peeked), testCase.expect)
			}
			parsed, n, err := varint.Parse(peeked)
			if err != nil {
				t.Fatalf("varint.Parse() error = %v", err)
			}
			if parsed != testCase.value {
				t.Fatalf("varint.Parse() value = %d, want %d", parsed, testCase.value)
			}
			if n != testCase.expect {
				t.Fatalf("varint.Parse() len = %d, want %d", n, testCase.expect)
			}
		})
	}
}

func TestPeekFirstVarintTooShort(t *testing.T) {
	reader := bufio.NewReader(bytes.NewReader([]byte{0x80}))

	_, err := peekFirstVarint(reader)
	if err == nil {
		t.Fatal("peekFirstVarint() error = nil, want error")
	}
}
