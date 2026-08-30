package moqtransport

import (
	"fmt"

	"github.com/mengelbart/moqtransport/internal/wire"
)

type Subgroup struct {
	stream     controlMessageWriter
	groupID    uint64
	subgroupID uint64

	firstObject  bool
	lastObjectID uint64
}

func newSubgroup(stream controlMessageWriter, trackAlias, groupID, subgroupID uint64, publisherPriority uint8) (*Subgroup, error) {
	if err := stream.Write(wire.NewSubgroupHeader(trackAlias, groupID, subgroupID, publisherPriority)); err != nil {
		return nil, err
	}
	return &Subgroup{
		stream:      stream,
		groupID:     groupID,
		subgroupID:  subgroupID,
		firstObject: true,
	}, nil
}

// WriteObject writes an object to the subgroup. Object IDs must increase
// strictly monotonically along the subgroup.
func (s *Subgroup) WriteObject(objectID uint64, payload []byte) (int, error) {
	delta := objectID
	if !s.firstObject {
		if objectID <= s.lastObjectID {
			return 0, fmt.Errorf("object ID %v not greater than previous object ID %v", objectID, s.lastObjectID)
		}
		delta = objectID - s.lastObjectID - 1
	}
	o := &wire.ObjectStream{
		ObjectIDDelta: delta,
		ObjectPayload: payload,
	}
	if err := s.stream.Write(o); err != nil {
		return 0, err
	}
	s.firstObject = false
	s.lastObjectID = objectID
	return len(payload), nil
}

// Close closes the subgroup.
func (s *Subgroup) Close() error {
	// TODO
	return nil
}
