package wire

import (
	"bytes"
	"io"
	"testing"

	"github.com/mengelbart/moqtransport/varint"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchStreamBytes(t *testing.T) {
	var buf bytes.Buffer
	appender := NewAppender(&buf, 18)

	require.NoError(t, appender.Write(&FetchHeader{RequestID: 42}))

	first := &FetchObject{
		GroupIDDelta:      1,
		SubgroupID:        9,
		ObjectIDDelta:     0,
		PublisherPriority: 200,
		ObjectPayload:     []byte("ab"),
	}
	first.SetHasGroupIDDelta(true)
	first.SetSubgroupIDMode(FetchSubgroupIDModeExplicit)
	first.SetHasObjectIDDelta(true)
	first.SetHasPriority(true)
	require.NoError(t, appender.Write(first))

	require.NoError(t, appender.Write(&FetchObject{ObjectPayload: []byte("c")}))
	require.NoError(t, appender.Write(NewEndOfNonExistentRange(2, 5)))

	assert.Equal(t, []byte{
		0x05, // FETCH_HEADER
		0x2a, // request ID

		0x1f, // flags: group delta, subgroup ID mode 0b11, object delta, priority
		0x01, // group ID delta
		0x09, // subgroup ID
		0x00, // object ID delta
		200,  // publisher priority
		0x02, 'a', 'b',

		0x00, // flags: subgroup ID mode 0b00, nothing present
		0x01, 'c',

		0x80, 0x8c, // flags: end of non-existent range
		0x02, // group ID delta
		0x05, // object ID delta
		0x00, // payload length
	}, buf.Bytes())
}

// fetchObjects covers every serialization flag, one combination per object.
func fetchObjects() []*FetchObject {
	explicit := &FetchObject{
		GroupIDDelta:      1,
		SubgroupID:        9,
		ObjectIDDelta:     0,
		PublisherPriority: 200,
		ObjectPayload:     []byte("hello"),
	}
	explicit.SetHasGroupIDDelta(true)
	explicit.SetSubgroupIDMode(FetchSubgroupIDModeExplicit)
	explicit.SetHasObjectIDDelta(true)
	explicit.SetHasPriority(true)

	prior := &FetchObject{ObjectIDDelta: 3, ObjectPayload: []byte("world")}
	prior.SetSubgroupIDMode(FetchSubgroupIDModePrior)
	prior.SetHasObjectIDDelta(true)

	priorPlusOne := &FetchObject{ObjectPayload: []byte("!")}
	priorPlusOne.SetSubgroupIDMode(FetchSubgroupIDModePriorPlusOne)

	properties := &FetchObject{
		Properties: []KeyValuePair{
			{Type: 1, Bytes: []byte("A")},
			{Type: 2, Varint: 42},
		},
		ObjectPayload: []byte("ab"),
	}
	properties.SetHasProperties(true)

	// A datagram object has no subgroup ID, the mode bits are ignored.
	datagram := &FetchObject{ObjectPayload: []byte("d")}
	datagram.SetDatagram(true)
	datagram.SetSubgroupIDMode(FetchSubgroupIDModeExplicit)

	return []*FetchObject{
		explicit,
		prior,
		priorPlusOne,
		properties,
		datagram,
		{}, // an empty object, every field absent
		NewEndOfNonExistentRange(2, 5),
		NewEndOfUnknownRange(0, 1),
	}
}

func TestFetchStreamRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	appender := NewAppender(&buf, 18)

	header := &FetchHeader{RequestID: 42}
	require.NoError(t, appender.Write(header))

	objects := fetchObjects()
	for _, o := range objects {
		require.NoError(t, appender.Write(o))
	}

	parser := NewParser(&buf, 18, StreamTypeData)

	msg, err := parser.Read()
	require.NoError(t, err)
	assert.Equal(t, header, msg)

	for i, want := range objects {
		msg, err := parser.Read()
		require.NoError(t, err, "object %v", i)
		assert.Equal(t, want, msg, "object %v", i)
	}

	_, err = parser.Read()
	assert.ErrorIs(t, err, io.EOF)
}

func TestParseInvalidSerializationFlags(t *testing.T) {
	cases := map[string]struct {
		flags  uint64
		fields []byte
	}{
		// No field is present.
		"128": {flags: 0x80, fields: []byte{0x00}},
		// No field is present.
		"256": {flags: 0x100, fields: []byte{0x00}},
		// Group delta, object delta, priority, properties and a payload length.
		// The datagram bit suppresses the subgroup ID.
		"255": {flags: 0xff, fields: []byte{0x00, 0x00, 0x00, 0x00, 0x00}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			buf := []byte{0x05, 0x2a} // FETCH_HEADER
			buf = varint.Append(buf, tc.flags)
			buf = append(buf, tc.fields...)

			parser := NewParser(bytes.NewReader(buf), 18, StreamTypeData)

			_, err := parser.Read()
			require.NoError(t, err)

			_, err = parser.Read()
			assert.ErrorIs(t, err, errInvalidSerializationFlags)
		})
	}
}

func TestParseTruncatedFetchObject(t *testing.T) {
	full := []byte{
		0x05, 0x2a, // FETCH_HEADER
		0x3f,            // flags: group delta, subgroup mode 0b11, object delta, priority, properties
		0x01,            // group ID delta
		0x09,            // subgroup ID
		0x00,            // object ID delta
		200,             // publisher priority
		0x03,            // properties length in bytes
		0x01, 0x01, 'A', // property type delta 1, type 1: one byte, 'A'
		0x02, 'a', 'b', // payload
	}
	for i := 3; i < len(full); i++ {
		parser := NewParser(bytes.NewReader(full[:i]), 18, StreamTypeData)

		_, err := parser.Read()
		require.NoError(t, err, "truncated after %v bytes", i)

		_, err = parser.Read()
		assert.ErrorIs(t, err, io.ErrUnexpectedEOF, "truncated after %v bytes", i)
	}
}

func TestParseFetchObjectLargePropertiesLength(t *testing.T) {
	buf := []byte{0x05, 0x2a} // FETCH_HEADER
	buf = append(buf, 0x20)   // flags: properties present
	buf = varint.Append(buf, 1<<40)

	parser := NewParser(bytes.NewReader(buf), 18, StreamTypeData)

	_, err := parser.Read()
	require.NoError(t, err)

	_, err = parser.Read()
	assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
}
