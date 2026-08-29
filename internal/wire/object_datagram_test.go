package wire

import (
	"io"
	"testing"

	"github.com/mengelbart/moqtransport/varint"
	"github.com/stretchr/testify/assert"
)

func TestObjectDatagramParseLargePropertyCount(t *testing.T) {
	// typ bit 0 set: the datagram claims to carry properties.
	data := []byte{0x01, 0x00, 0x00, 0x00, 0x00}
	data = varint.Append(data, 1<<40)

	m := ObjectDatagram{}
	parsed, err := m.Parse(data)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, len(data), parsed)
	assert.Empty(t, m.Properties)
}

func TestObjectDatagramParseTruncatedProperty(t *testing.T) {
	data := []byte{0x01, 0x00, 0x00, 0x00, 0x00}
	data = varint.Append(data, 2)
	data = varint.Append(data, 1) // property type 1: length-prefixed bytes
	data = varint.Append(data, 8) // claims 8 bytes
	data = append(data, 'A')

	m := ObjectDatagram{}
	_, err := m.Parse(data)
	assert.Equal(t, io.ErrUnexpectedEOF, err)
	assert.Empty(t, m.Properties)
}

func TestObjectDatagramParseProperties(t *testing.T) {
	data := []byte{0x01, 0x04, 0x05, 0x06, 0x07}
	data = varint.Append(data, 2)
	data = varint.Append(data, 2) // property type 2: varint value
	data = varint.Append(data, 42)
	data = varint.Append(data, 1) // property type 1: length-prefixed bytes
	data = varint.Append(data, 1)
	data = append(data, 'A')
	data = append(data, "payload"...)

	m := ObjectDatagram{}
	parsed, err := m.Parse(data)
	assert.NoError(t, err)
	assert.Equal(t, len(data), parsed)
	assert.Equal(t, uint64(4), m.TrackAlias)
	assert.Equal(t, uint64(5), m.GroupID)
	assert.Equal(t, uint64(6), m.ObjectID)
	assert.Equal(t, uint8(7), m.PublisherPriority)
	assert.Equal(t, []KeyValuePair{
		{Type: 2, Varint: 42},
		{Type: 1, Bytes: []byte("A")},
	}, m.Properties)
	assert.Equal(t, []byte("payload"), m.ObjectPayload)
}
