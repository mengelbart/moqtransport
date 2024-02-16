package moqtransport

type groupHeaderStreamObject struct {
	stream   SendStream
	objectID uint64
}

func (s *groupHeaderStreamObject) Write(payload []byte) (int, error) {
	shgo := streamHeaderGroupObject{
		ObjectID:      s.objectID,
		ObjectPayload: payload,
	}
	buf := make([]byte, 0, 16+len(payload))
	buf = shgo.append(buf)
	return s.stream.Write(buf)
}
