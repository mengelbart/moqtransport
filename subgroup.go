package moqtransport

import (
	"github.com/mengelbart/moqtransport/internal/wire"
)

type Subgroup struct {
	stream     controlMessageWriter
	groupID    uint64
	subgroupID uint64
}

func newSubgroup(stream controlMessageWriter, trackAlias, groupID, subgroupID uint64, publisherPriority uint8) (*Subgroup, error) {
	shgm := &wire.SubgroupHeader{
		TrackAlias:        trackAlias,
		GroupID:           groupID,
		SubgroupID:        subgroupID,
		PublisherPriority: publisherPriority,
	}
	if err := stream.Write(shgm); err != nil {
		return nil, err
	}
	return &Subgroup{
		stream:     stream,
		groupID:    groupID,
		subgroupID: subgroupID,
	}, nil
}

func (s *Subgroup) WriteObject(objectID uint64, payload []byte) (int, error) {
	o := &wire.ObjectStream{
		ObjectPayload: payload,
	}
	if err := s.stream.Write(o); err != nil {
		return 0, err
	}
	return len(payload), nil
}

// Close closes the subgroup.
func (s *Subgroup) Close() error {
	// TODO
	return nil
}
