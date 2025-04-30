package moqtransport

import (
	"github.com/mengelbart/moqtransport/internal/wire"
	"github.com/mengelbart/qlog"
	"github.com/mengelbart/qlog/moqt"
)

type FetchStream struct {
	stream  SendStream
	qlogger *qlog.Logger
}

func newFetchStream(stream SendStream, requestID uint64, qlogger *qlog.Logger) (*FetchStream, error) {
	fhm := &wire.FetchHeaderMessage{
		RequestID: requestID,
	}
	buf := make([]byte, 0, 24)
	buf = fhm.Append(buf)
	_, err := stream.Write(buf)
	if err != nil {
		return nil, err
	}
	if qlogger != nil {
		qlogger.Log(moqt.StreamTypeSetEvent{
			Owner:      moqt.GetOwner(moqt.OwnerLocal),
			StreamID:   stream.StreamID(),
			StreamType: moqt.StreamTypeFetchHeader,
		})
	}
	return &FetchStream{
		stream:  stream,
		qlogger: qlogger,
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
	if f.qlogger != nil {
		f.qlogger.Log(moqt.FetchObjectEvent{
			EventName:              moqt.FetchObjectEventCreated,
			StreamID:               f.stream.StreamID(),
			GroupID:                groupID,
			SubgroupID:             subgroupID,
			ObjectID:               objectID,
			ExtensionHeadersLength: 0,
			ExtensionHeaders:       nil,
			ObjectPayloadLength:    uint64(len(payload)),
			ObjectStatus:           0,
			ObjectPayload: qlog.RawInfo{
				Length:        uint64(len(payload)),
				PayloadLength: uint64(len(payload)),
				Data:          payload,
			},
		})
	}
	return len(payload), nil
}

func (f *FetchStream) Close() error {
	return f.stream.Close()
}
