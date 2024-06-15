package wire

import "io"

type messageReader interface {
	io.Reader
	io.ByteReader
	Discard(int) (int, error)
}

type Message interface {
	Append([]byte) []byte
	parse(messageReader) error
}

type ObjectMessageType uint64

// Object message types
const (
	ObjectStreamMessageType      ObjectMessageType = 0x00
	ObjectDatagramMessageType    ObjectMessageType = 0x01
	StreamHeaderTrackMessageType ObjectMessageType = 0x50
	StreamHeaderGroupMessageType ObjectMessageType = 0x51
)

func (mt ObjectMessageType) String() string {
	switch mt {
	case ObjectStreamMessageType:
		return "ObjectStreamMessage"
	case ObjectDatagramMessageType:
		return "objectDatagram"
	case StreamHeaderTrackMessageType:
		return "StreamHeaderTrackMessage"
	case StreamHeaderGroupMessageType:
		return "streamHeaderGroupMessage"
	}
	return "unknown message type"
}

type controlMessageType uint64

// Control message types
const (
	subscribeUpdateMessageType    controlMessageType = 0x02
	subscribeMessageType          controlMessageType = 0x03
	subscribeOkMessageType        controlMessageType = 0x04
	subscribeErrorMessageType     controlMessageType = 0x05
	announceMessageType           controlMessageType = 0x06
	announceOkMessageType         controlMessageType = 0x07
	announceErrorMessageType      controlMessageType = 0x08
	unannounceMessageType         controlMessageType = 0x09
	unsubscribeMessageType        controlMessageType = 0x0a
	subscribeDoneMessageType      controlMessageType = 0x0b
	announceCancelMessageType     controlMessageType = 0x0c
	trackStatusRequestMessageType controlMessageType = 0x0d
	trackStatusMessageType        controlMessageType = 0x0e
	goAwayMessageType             controlMessageType = 0x10
	clientSetupMessageType        controlMessageType = 0x40
	serverSetupMessageType        controlMessageType = 0x41
)

func (mt controlMessageType) String() string {
	switch mt {
	case subscribeUpdateMessageType:
		return "SubscribeUpdateMessage"
	case subscribeMessageType:
		return "SubscribeMessage"
	case subscribeOkMessageType:
		return "SubscribeOkMessage"
	case subscribeErrorMessageType:
		return "SubscribeErrorMessage"
	case announceMessageType:
		return "AnnounceMessage"
	case announceOkMessageType:
		return "AnnounceOkMessage"
	case announceErrorMessageType:
		return "AnnounceErrorMessage"
	case unannounceMessageType:
		return "AnannounceMessage"
	case unsubscribeMessageType:
		return "UnsubscribeMessage"
	case subscribeDoneMessageType:
		return "SubscribeDoneMessage"
	case announceCancelMessageType:
		return "AnnounceCancelMessage"
	case trackStatusRequestMessageType:
		return "TrackStatusRequestMessage"
	case trackStatusMessageType:
		return "TrackStatusMessage"
	case goAwayMessageType:
		return "GoAwayMessage"
	case clientSetupMessageType:
		return "ClientSetupMessage"
	case serverSetupMessageType:
		return "ServerSetupMessage"
	}
	return "unknown message type"
}
