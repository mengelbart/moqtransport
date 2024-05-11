package moqtransport

type groupHeaderStream struct {
	stream SendStream
}

func newGroupHeaderStream(stream SendStream, subscribeID, trackAlias, groupID, objectSendOrder uint64) (*groupHeaderStream, error) {
	shgm := &streamHeaderGroupMessage{
		SubscribeID:     subscribeID,
		TrackAlias:      trackAlias,
		GroupID:         groupID,
		ObjectSendOrder: objectSendOrder,
	}
	buf := make([]byte, 0, 40)
	buf = shgm.append(buf)
	_, err := stream.Write(buf)
	if err != nil {
		return nil, err
	}
	return &groupHeaderStream{
		stream: stream,
	}, nil
}

func (s *groupHeaderStream) writeObject(objectID uint64, payload []byte) (int, error) {
	shgo := streamHeaderGroupObject{
		ObjectID:      objectID,
		ObjectPayload: payload,
	}
	buf := make([]byte, 0, 16+len(payload))
	buf = shgo.append(buf)
	return s.stream.Write(buf)
}

func (s *groupHeaderStream) Close() error {
	return s.stream.Close()
}
