package moqtransport

type trackHeaderStreamObject struct {
	stream            sendStream
	groupID, objectID uint64
}

func (o *trackHeaderStreamObject) Write(payload []byte) (int, error) {
	shto := streamHeaderTrackObject{
		GroupID:       o.groupID,
		ObjectID:      o.objectID,
		ObjectPayload: payload,
	}
	buf := make([]byte, 0, 24+len(payload))
	buf = shto.append(buf)
	return o.stream.Write(buf)
}
