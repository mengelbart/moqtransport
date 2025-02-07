package wire

import (
	"io"
)

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
	messageTypeSubscribeAnnounces      controlMessageType = 0x11
	messageTypeSubscribeAnnouncesOk    controlMessageType = 0x12
	messageTypeSubscribeAnnouncesError controlMessageType = 0x13
	messageTypeUnsubscribeAnnounces    controlMessageType = 0x14
	messageTypeMaxSubscribeID          controlMessageType = 0x15
	messageTypeFetch                   controlMessageType = 0x16
	messageTypeFetchCancel             controlMessageType = 0x17
	messageTypeFetchOk                 controlMessageType = 0x18
	messageTypeFetchError              controlMessageType = 0x19
	messageTypeSubscribesBlocked       controlMessageType = 0x1a
	messageTypeClientSetup             controlMessageType = 0x40
	messageTypeServerSetup             controlMessageType = 0x41
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
	case messageTypeFetch:
		return "FetchMessage"
	case messageTypeFetchCancel:
		return "FetchCancelMessage"
	case messageTypeFetchError:
		return "FetchErrorMessage"
	case messageTypeFetchOk:
		return "FetchOkMessage"
	case messageTypeMaxSubscribeID:
		return "MaxSubscribeIDMessage"
	case messageTypeSubscribeAnnounces:
		return "SubscribeAnnouncesMessage"
	case messageTypeSubscribeAnnouncesError:
		return "SubscribeAnnouncesErrorMessage"
	case messageTypeSubscribeAnnouncesOk:
		return "SubscribeAnnouncesOkMessage"
	case messageTypeSubscribesBlocked:
		return "SubscribesBlockedMessage"
	case messageTypeUnsubscribeAnnounces:
		return "UnsubscribeAnnouncesMessage"
	}
	return "unknown message type"
}

type messageReader interface {
	io.Reader
	io.ByteReader
	Discard(int) (int, error)
}

type Message interface {
	Append([]byte) []byte
	parse([]byte) error
}

type ControlMessage interface {
	Message
	Type() controlMessageType
}
