package moqtransport

import (
	"testing"

	"github.com/mengelbart/moqtransport/internal/wire"
	"github.com/stretchr/testify/assert"
)

func TestKVPList_ToWire(t *testing.T) {
	t.Run("ToWire creates independent copy", func(t *testing.T) {
		original := KVPList{
			{Type: 1, ValueVarInt: 100},
			{Type: 2, ValueBytes: []byte("test")},
		}

		wireCopy := original.ToWire()

		// Modify the original
		original[0].ValueVarInt = 200
		original[1].ValueBytes[0] = 'X'

		// Wire copy should be unchanged
		assert.Equal(t, uint64(100), wireCopy[0].ValueVarInt)
		assert.Equal(t, []byte("test"), wireCopy[1].ValueBytes)
	})

	t.Run("ToWire handles nil slice", func(t *testing.T) {
		var original KVPList
		wireCopy := original.ToWire()
		assert.Nil(t, wireCopy)
	})

	t.Run("ToWire handles empty slice", func(t *testing.T) {
		original := KVPList{}
		wireCopy := original.ToWire()
		assert.NotNil(t, wireCopy)
		assert.Len(t, wireCopy, 0)
	})
}

func TestFromWire(t *testing.T) {
	t.Run("FromWire creates independent copy", func(t *testing.T) {
		original := wire.KVPList{
			{Type: 1, ValueVarInt: 100},
			{Type: 2, ValueBytes: []byte("test")},
		}

		kvpCopy := FromWire(original)

		// Modify the original
		original[0].ValueVarInt = 200
		original[1].ValueBytes[0] = 'X'

		// KVP copy should be unchanged
		assert.Equal(t, uint64(100), kvpCopy[0].ValueVarInt)
		assert.Equal(t, []byte("test"), kvpCopy[1].ValueBytes)
	})

	t.Run("FromWire handles nil slice", func(t *testing.T) {
		var original wire.KVPList
		kvpCopy := FromWire(original)
		assert.Nil(t, kvpCopy)
	})

	t.Run("FromWire handles empty slice", func(t *testing.T) {
		original := wire.KVPList{}
		kvpCopy := FromWire(original)
		assert.NotNil(t, kvpCopy)
		assert.Len(t, kvpCopy, 0)
	})
}

func TestKVPList_RoundTrip(t *testing.T) {
	t.Run("ToWire and FromWire preserve data", func(t *testing.T) {
		original := KVPList{
			{Type: 1, ValueVarInt: 100},
			{Type: 2, ValueBytes: []byte("test")},
			{Type: 3, ValueVarInt: 42, ValueBytes: []byte("both")},
		}

		// Convert to wire and back
		roundTrip := FromWire(original.ToWire())

		assert.Equal(t, original, roundTrip)
	})
}