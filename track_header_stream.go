package moqtransport

type TrackHeaderStream struct {
	stream SendStream
}

func newTrackHeaderStream(stream SendStream, subscribeID, trackAlias, objectSendOrder uint64) (*TrackHeaderStream, error) {
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
	return &TrackHeaderStream{
		stream: stream,
	}, nil
}

func (s *TrackHeaderStream) NewObject(groupID, objectID uint64) *TrackHeaderStreamObject {
	return &TrackHeaderStreamObject{
		stream:   s.stream,
		groupID:  groupID,
		objectID: objectID,
	}
}

func (s *TrackHeaderStream) Close() error {
	return s.stream.Close()
}
