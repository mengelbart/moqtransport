package moqtransport

type groupHeaderStream struct {
	stream sendStream
}

func newGroupHeaderStream(stream sendStream, subscribeID, trackAlias, groupID, objectSendOrder uint64) (*groupHeaderStream, error) {
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

func (s *groupHeaderStream) NewObject() *groupHeaderStreamObject {
	return &groupHeaderStreamObject{
		stream: s.stream,
	}
}

func (s *groupHeaderStream) Close() error {
	return s.stream.Close()
}
