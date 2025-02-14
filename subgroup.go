package moqtransport

import (
	"github.com/mengelbart/moqtransport/internal/wire"
)

type Subgroup struct {
	stream SendStream
}

func newSubgroup(stream SendStream, trackAlias, groupID, subgroupID uint64, publisherPriority uint8) (*Subgroup, error) {
	shgm := &wire.StreamHeaderSubgroupMessage{
		TrackAlias:        trackAlias,
		GroupID:           groupID,
		SubgroupID:        subgroupID,
		PublisherPriority: publisherPriority,
	}
	buf := make([]byte, 0, 40)
	buf = shgm.Append(buf)
	_, err := stream.Write(buf)
	if err != nil {
		return nil, err
	}
	return &Subgroup{
		stream: stream,
	}, nil
}

func (s *Subgroup) WriteObject(objectID uint64, payload []byte) (int, error) {
	var buf []byte
	if len(payload) > 0 {
		buf = make([]byte, 0, 16+len(payload))
	} else {
		buf = make([]byte, 0, 24)
	}
	o := wire.ObjectMessage{
		ObjectID:      objectID,
		ObjectPayload: payload,
	}
	buf = o.AppendSubgroup(buf)
	_, err := s.stream.Write(buf)
	if err != nil {
		return 0, err
	}
	return len(payload), nil
}

// Close closes the subgroup.
func (s *Subgroup) Close() error {
	return s.stream.Close()
}
