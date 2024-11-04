package wire

import "io"

type messageReader interface {
	io.Reader
	io.ByteReader
	Discard(int) (int, error)
}

type Message interface {
	Type() controlMessageType
	Append([]byte) []byte
	parse([]byte) error
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
	messageTypeSubscribeUpdate         controlMessageType = 0x02
	messageTypeSubscribe               controlMessageType = 0x03
	messageTypeSubscribeOk             controlMessageType = 0x04
	messageTypeSubscribeError          controlMessageType = 0x05
	messageTypeAnnounce                controlMessageType = 0x06
	messageTypeAnnounceOk              controlMessageType = 0x07
	messageTypeAnnounceError           controlMessageType = 0x08
	messageTypeUnannounce              controlMessageType = 0x09
	messageTypeUnsubscribe             controlMessageType = 0x0a
	messageTypeSubscribeDone           controlMessageType = 0x0b
	messageTypeAnnounceCancel          controlMessageType = 0x0c
	messageTypeTrackStatusRequest      controlMessageType = 0x0d
	messageTypeTrackStatus             controlMessageType = 0x0e
	messageTypeGoAway                  controlMessageType = 0x10
	messageTypeSubscribeAnnounces      controlMessageType = 0x00
	messageTypeSubscribeAnnouncesOk    controlMessageType = 0x00
	messageTypeSubscribeAnnouncesError controlMessageType = 0x00
	messageTypeUnsubscribeAnnounces    controlMessageType = 0x00
	messageTypeMaxSubscribeID          controlMessageType = 0x00

	messageTypeFetch       controlMessageType = 0x00
	messageTypeFetchCancel controlMessageType = 0x00
	messageTypeFetchOk     controlMessageType = 0x00

	messageTypeFetchError  controlMessageType = 0x00
	messageTypeClientSetup controlMessageType = 0x40
	messageTypeServerSetup controlMessageType = 0x41
)

func (mt controlMessageType) String() string {
	switch mt {
	case messageTypeSubscribeUpdate:
		return "SubscribeUpdateMessage"
	case messageTypeSubscribe:
		return "SubscribeMessage"
	case messageTypeSubscribeOk:
		return "SubscribeOkMessage"
	case messageTypeSubscribeError:
		return "SubscribeErrorMessage"
	case messageTypeAnnounce:
		return "AnnounceMessage"
	case messageTypeAnnounceOk:
		return "AnnounceOkMessage"
	case messageTypeAnnounceError:
		return "AnnounceErrorMessage"
	case messageTypeUnannounce:
		return "AnannounceMessage"
	case messageTypeUnsubscribe:
		return "UnsubscribeMessage"
	case messageTypeSubscribeDone:
		return "SubscribeDoneMessage"
	case messageTypeAnnounceCancel:
		return "AnnounceCancelMessage"
	case messageTypeTrackStatusRequest:
		return "TrackStatusRequestMessage"
	case messageTypeTrackStatus:
		return "TrackStatusMessage"
	case messageTypeGoAway:
		return "GoAwayMessage"
	case messageTypeClientSetup:
		return "ClientSetupMessage"
	case messageTypeServerSetup:
		return "ServerSetupMessage"
	}
	return "unknown message type"
}
