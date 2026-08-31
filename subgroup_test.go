package moqtransport

import (
	"bytes"
	"testing"

	"github.com/mengelbart/moqtransport/internal/wire"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubgroupWriteObject(t *testing.T) {
	var buf bytes.Buffer
	subgroup, err := newSubgroup(wire.NewAppender(&buf, 18), 4, 7, 9, 200)
	require.NoError(t, err)

	for _, objectID := range []uint64{0, 1, 5} {
		n, err := subgroup.WriteObject(objectID, []byte("payload"))
		require.NoError(t, err)
		assert.Equal(t, len("payload"), n)
	}

	parser := wire.NewParser(&buf, 18, wire.StreamTypeData)

	msg, err := parser.Read()
	require.NoError(t, err)
	header, ok := msg.(*wire.SubgroupHeader)
	require.True(t, ok)
	assert.Equal(t, uint64(4), header.TrackAlias)
	assert.Equal(t, uint64(7), header.GroupID)
	assert.Equal(t, uint64(9), header.SubgroupID)
	assert.Equal(t, uint8(200), header.PublisherPriority)

	for _, want := range []uint64{0, 0, 3} {
		msg, err := parser.Read()
		require.NoError(t, err)
		o, ok := msg.(*wire.SubgroupObject)
		require.True(t, ok)
		assert.Equal(t, want, o.ObjectIDDelta)
	}
}

func TestSubgroupWriteObjectOutOfOrder(t *testing.T) {
	var buf bytes.Buffer
	subgroup, err := newSubgroup(wire.NewAppender(&buf, 18), 4, 7, 9, 0)
	require.NoError(t, err)

	_, err = subgroup.WriteObject(3, []byte("payload"))
	require.NoError(t, err)

	_, err = subgroup.WriteObject(3, []byte("payload"))
	assert.Error(t, err)

	_, err = subgroup.WriteObject(2, []byte("payload"))
	assert.Error(t, err)
}
