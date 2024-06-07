package moqtransport

type trackHeaderStream struct {
	stream SendStream
}

func newTrackHeaderStream(stream SendStream, subscribeID, trackAlias, objectSendOrder uint64) (*trackHeaderStream, error) {
	shtm := &streamHeaderTrackMessage{
		SubscribeID:     subscribeID,
		TrackAlias:      trackAlias,
		ObjectSendOrder: objectSendOrder,
	}
	buf := make([]byte, 0, 32)
	buf = shtm.append(buf)
	_, err := stream.Write(buf)
	if err != nil {
		return nil, err
	}
	return &trackHeaderStream{
		stream: stream,
	}, nil
}

func (s *trackHeaderStream) writeObject(groupID, objectID uint64, payload []byte) (int, error) {
	shto := streamHeaderTrackObject{
		GroupID:       groupID,
		ObjectID:      objectID,
		ObjectPayload: payload,
	}
	buf := make([]byte, 0, 32+len(payload))
	buf = shto.append(buf)
	return s.stream.Write(buf)
}

func (s *trackHeaderStream) Close() error {
	return s.stream.Close()
}
