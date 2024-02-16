package moqtransport

type objectStream struct {
	stream SendStream
}

func newObjectStream(stream SendStream, subscribeID, trackAlias, groupID, objectID, objectSendOrder uint64) (*objectStream, error) {
	osm := &objectMessage{
		preferDatagram:  false,
		SubscribeID:     subscribeID,
		TrackAlias:      trackAlias,
		GroupID:         groupID,
		ObjectID:        objectID,
		ObjectSendOrder: objectSendOrder,
		ObjectPayload:   nil,
	}
	buf := make([]byte, 0, 48)
	buf = osm.append(buf)
	_, err := stream.Write(buf)
	if err != nil {
		return nil, err
	}
	return &objectStream{
		stream: stream,
	}, nil
}

func (s *objectStream) Write(payload []byte) (int, error) {
	return s.stream.Write(payload)
}

func (s *objectStream) Close() error {
	return s.stream.Close()
}
