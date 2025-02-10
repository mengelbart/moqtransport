package moqtransport

import "github.com/mengelbart/moqtransport/internal/wire"

type FetchStream struct {
	stream SendStream
}

func newFetchStream(stream SendStream, subscribeID uint64) (*FetchStream, error) {
	fhm := &wire.FetchHeaderMessage{
		SubscribeID: subscribeID,
	}
	buf := make([]byte, 0, 24)
	buf = fhm.Append(buf)
	_, err := stream.Write(buf)
	if err != nil {
		return nil, err
	}
	return &FetchStream{
		stream: stream,
	}, nil
}

func (f *FetchStream) WriteObject(
	groupID, subgroupID, objectID uint64,
	priority uint8,
	payload []byte,
) (int, error) {
	buf := make([]byte, 0, 1400)
	fo := wire.ObjectMessage{
		GroupID:           groupID,
		SubgroupID:        subgroupID,
		ObjectID:          objectID,
		PublisherPriority: priority,
		ObjectStatus:      0,
		ObjectPayload:     payload,
	}
	buf = fo.AppendFetch(buf)
	_, err := f.stream.Write(buf)
	if err != nil {
		return 0, err
	}
	return len(payload), nil
}

func (f *FetchStream) Close() error {
	return f.stream.Close()
}
