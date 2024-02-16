package moqtransport

type TrackHeaderStreamObject struct {
	stream            SendStream
	groupID, objectID uint64
}

func (o *TrackHeaderStreamObject) Write(payload []byte) (int, error) {
	shto := streamHeaderTrackObject{
		GroupID:       o.groupID,
		ObjectID:      o.objectID,
		ObjectPayload: payload,
	}
	buf := make([]byte, 0, 32+len(payload))
	buf = shto.append(buf)
	return o.stream.Write(buf)
}
