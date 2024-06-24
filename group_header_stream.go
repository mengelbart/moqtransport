package moqtransport

import "github.com/mengelbart/moqtransport/internal/wire"

type groupHeaderStream struct {
	stream SendStream
}

func newGroupHeaderStream(stream SendStream, subscribeID, trackAlias, groupID, objectSendOrder uint64) (*groupHeaderStream, error) {
	shgm := &wire.StreamHeaderGroupMessage{
		SubscribeID:     subscribeID,
		TrackAlias:      trackAlias,
		GroupID:         groupID,
		ObjectSendOrder: objectSendOrder,
	}
	buf := make([]byte, 0, 40)
	buf = shgm.Append(buf)
	_, err := stream.Write(buf)
	if err != nil {
		return nil, err
	}
	return &groupHeaderStream{
		stream: stream,
	}, nil
}

func (s *groupHeaderStream) writeObject(objectID uint64, payload []byte) (int, error) {
	shgo := wire.StreamHeaderGroupObject{
		ObjectID:      objectID,
		ObjectPayload: payload,
	}
	buf := make([]byte, 0, 16+len(payload))
	buf = shgo.Append(buf)
	return s.stream.Write(buf)
}

func (s *groupHeaderStream) Close() error {
	return s.stream.Close()
}

func (s *groupHeaderStream) Cancel() {
	s.stream.CancelWrite(0)
}
