package wire

import (
	"bytes"
	"testing"

	"github.com/mengelbart/moqtransport/varint"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubgroupStreamBytes(t *testing.T) {
	var buf bytes.Buffer
	appender := NewAppender(&buf, 18)

	require.NoError(t, appender.Write(NewSubgroupHeader(4, 7, 9, 200)))
	require.NoError(t, appender.Write(&ObjectStream{ObjectIDDelta: 0, ObjectPayload: []byte("ab")}))
	require.NoError(t, appender.Write(&ObjectStream{ObjectIDDelta: 3, ObjectPayload: []byte("c")}))

	assert.Equal(t, []byte{
		0x14, // type: bit 4 set, subgroup ID mode 0b10
		0x04, // track alias
		0x07, // group ID
		0x09, // subgroup ID
		200,  // publisher priority
		0x00, 0x02, 'a', 'b',
		0x03, 0x01, 'c',
	}, buf.Bytes())
}

func TestSubgroupStreamRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	appender := NewAppender(&buf, 18)

	header := NewSubgroupHeader(4, 7, 9, 200)
	require.NoError(t, appender.Write(header))

	objects := []*ObjectStream{
		{ObjectIDDelta: 0, ObjectPayload: []byte("hello")},
		{ObjectIDDelta: 3, ObjectPayload: []byte("world")},
		{ObjectIDDelta: 0, ObjectStatus: 3},
	}
	for _, o := range objects {
		require.NoError(t, appender.Write(o))
	}

	parser := NewParser(&buf, 18, StreamTypeData)

	msg, err := parser.Read()
	require.NoError(t, err)
	assert.Equal(t, header, msg)

	for _, want := range objects {
		msg, err := parser.Read()
		require.NoError(t, err)
		assert.Equal(t, want, msg)
	}
}

func TestSubgroupStreamDefaultPriority(t *testing.T) {
	var buf bytes.Buffer

	header := NewSubgroupHeader(4, 7, 9, 200)
	header.SetDefaultPriority(true)
	require.NoError(t, NewAppender(&buf, 18).Write(header))

	assert.Equal(t, []byte{0x34, 0x04, 0x07, 0x09}, buf.Bytes())

	msg, err := NewParser(&buf, 18, StreamTypeData).Read()
	require.NoError(t, err)
	assert.Equal(t, &SubgroupHeader{typ: 0x34, TrackAlias: 4, GroupID: 7, SubgroupID: 9}, msg)
}

func TestFetchHeaderRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, NewAppender(&buf, 18).Write(&FetchHeader{RequestID: 42}))

	assert.Equal(t, []byte{0x05, 0x2a}, buf.Bytes())

	msg, err := NewParser(&buf, 18, StreamTypeData).Read()
	require.NoError(t, err)
	assert.Equal(t, &FetchHeader{RequestID: 42}, msg)
}

func TestParseInvalidDataStreamType(t *testing.T) {
	cases := map[string]uint64{
		"reserved subgroup id mode": 0x16,
		"bit 4 not set":             0x00,
		"bit 7 set":                 0x90,
		// Truncating the type to its low byte would make this look like 0x10.
		"multi byte type": 0x110,
	}
	for name, typ := range cases {
		t.Run(name, func(t *testing.T) {
			buf := varint.Append(nil, typ)
			buf = append(buf, 0x04, 0x07, 0x09, 0x00)

			_, err := NewParser(bytes.NewReader(buf), 18, StreamTypeData).Read()
			assert.Error(t, err)
		})
	}
}

func TestParseTruncatedSubgroupHeader(t *testing.T) {
	full := []byte{0x14, 0x04, 0x07, 0x09, 200}
	for i := 1; i < len(full); i++ {
		_, err := NewParser(bytes.NewReader(full[:i]), 18, StreamTypeData).Read()
		assert.Error(t, err, "truncated after %v bytes", i)
	}
}
