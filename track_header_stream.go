package moqtransport

type trackHeaderStream struct {
	stream sendStream
}

func newTrackHeaderStream(stream sendStream, subscribeID, trackAlias, objectSendOrder uint64) (*trackHeaderStream, error) {
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

func (s *trackHeaderStream) NewObject(groupID, objectID uint64) *trackHeaderStreamObject {
	return &trackHeaderStreamObject{
		stream:   s.stream,
		groupID:  groupID,
		objectID: objectID,
	}
}

func (s *trackHeaderStream) Close() error {
	return s.stream.Close()
}
