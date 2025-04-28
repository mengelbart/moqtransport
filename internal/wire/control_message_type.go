package wire

import (
	"io"
	"log/slog"
)

type controlMessageType uint64

// Control message types
const (
	messageTypeClientSetup controlMessageType = 0x20
	messageTypeServerSetup controlMessageType = 0x21

	messageTypeGoAway controlMessageType = 0x10

	messageTypeMaxRequestID    controlMessageType = 0x15
	messageTypeRequestsBlocked controlMessageType = 0x1a

	messageTypeSubscribe       controlMessageType = 0x03
	messageTypeSubscribeOk     controlMessageType = 0x04
	messageTypeSubscribeError  controlMessageType = 0x05
	messageTypeUnsubscribe     controlMessageType = 0x0a
	messageTypeSubscribeUpdate controlMessageType = 0x02
	messageTypeSubscribeDone   controlMessageType = 0x0b

	messageTypeFetch       controlMessageType = 0x16
	messageTypeFetchOk     controlMessageType = 0x18
	messageTypeFetchError  controlMessageType = 0x19
	messageTypeFetchCancel controlMessageType = 0x17

	messageTypeTrackStatusRequest controlMessageType = 0x0d
	messageTypeTrackStatus        controlMessageType = 0x0e

	messageTypeAnnounce       controlMessageType = 0x06
	messageTypeAnnounceOk     controlMessageType = 0x07
	messageTypeAnnounceError  controlMessageType = 0x08
	messageTypeUnannounce     controlMessageType = 0x09
	messageTypeAnnounceCancel controlMessageType = 0x0c

	messageTypeSubscribeAnnounces      controlMessageType = 0x11
	messageTypeSubscribeAnnouncesOk    controlMessageType = 0x12
	messageTypeSubscribeAnnouncesError controlMessageType = 0x13
	messageTypeUnsubscribeAnnounces    controlMessageType = 0x14
)

func (mt controlMessageType) String() string {
	switch mt {
	case messageTypeClientSetup:
		return "ClientSetupMessage"
	case messageTypeServerSetup:
		return "ServerSetupMessage"

	case messageTypeGoAway:
		return "GoAwayMessage"

	case messageTypeMaxRequestID:
		return "MaxSubscribeIDMessage"
	case messageTypeRequestsBlocked:
		return "SubscribesBlockedMessage"

	case messageTypeSubscribe:
		return "SubscribeMessage"
	case messageTypeSubscribeOk:
		return "SubscribeOkMessage"
	case messageTypeSubscribeError:
		return "SubscribeErrorMessage"
	case messageTypeUnsubscribe:
		return "UnsubscribeMessage"
	case messageTypeSubscribeUpdate:
		return "SubscribeUpdateMessage"
	case messageTypeSubscribeDone:
		return "SubscribeDoneMessage"

	case messageTypeFetch:
		return "FetchMessage"
	case messageTypeFetchOk:
		return "FetchOkMessage"
	case messageTypeFetchError:
		return "FetchErrorMessage"
	case messageTypeFetchCancel:
		return "FetchCancelMessage"

	case messageTypeTrackStatusRequest:
		return "TrackStatusRequestMessage"
	case messageTypeTrackStatus:
		return "TrackStatusMessage"

	case messageTypeAnnounce:
		return "AnnounceMessage"
	case messageTypeAnnounceOk:
		return "AnnounceOkMessage"
	case messageTypeAnnounceError:
		return "AnnounceErrorMessage"
	case messageTypeUnannounce:
		return "AnannounceMessage"
	case messageTypeAnnounceCancel:
		return "AnnounceCancelMessage"

	case messageTypeSubscribeAnnounces:
		return "SubscribeAnnouncesMessage"
	case messageTypeSubscribeAnnouncesOk:
		return "SubscribeAnnouncesOkMessage"
	case messageTypeSubscribeAnnouncesError:
		return "SubscribeAnnouncesErrorMessage"
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
	parse(Version, []byte) error
}

type ControlMessage interface {
	Message
	Type() controlMessageType
	slog.LogValuer
}
