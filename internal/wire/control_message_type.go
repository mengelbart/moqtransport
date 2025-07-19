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
	messageTypeSubscribeUpdate controlMessageType = 0x02
	messageTypeUnsubscribe     controlMessageType = 0x0a
	messageTypeSubscribeDone   controlMessageType = 0x0b

	messageTypePublish      controlMessageType = 0x1d
	messageTypePublishOk    controlMessageType = 0x1e
	messageTypePublishError controlMessageType = 0x1f

	messageTypeFetch       controlMessageType = 0x16
	messageTypeFetchOk     controlMessageType = 0x18
	messageTypeFetchError  controlMessageType = 0x19
	messageTypeFetchCancel controlMessageType = 0x17

	messageTypeTrackStatus      controlMessageType = 0x0d
	messageTypeTrackStatusOk    controlMessageType = 0x0e
	messageTypeTrackStatusError controlMessageType = 0x0f

	messageTypeAnnounce       controlMessageType = 0x06
	messageTypeAnnounceOk     controlMessageType = 0x07
	messageTypeAnnounceError  controlMessageType = 0x08
	messageTypeUnannounce     controlMessageType = 0x09
	messageTypeAnnounceCancel controlMessageType = 0x0c

	messageTypeSubscribeNamespace      controlMessageType = 0x11
	messageTypeSubscribeNamespaceOk    controlMessageType = 0x12
	messageTypeSubscribeNamespaceError controlMessageType = 0x13
	messageTypeUnsubscribeNamespace    controlMessageType = 0x14
)

func (mt controlMessageType) String() string {
	switch mt {
	case messageTypeClientSetup:
		return "ClientSetup"
	case messageTypeServerSetup:
		return "ServerSetup"

	case messageTypeGoAway:
		return "GoAway"

	case messageTypeMaxRequestID:
		return "MaxRequestID"
	case messageTypeRequestsBlocked:
		return "RequestsBlocked"

	case messageTypeSubscribe:
		return "Subscribe"
	case messageTypeSubscribeOk:
		return "SubscribeOk"
	case messageTypeSubscribeError:
		return "SubscribeError"
	case messageTypeUnsubscribe:
		return "Unsubscribe"
	case messageTypeSubscribeUpdate:
		return "SubscribeUpdate"
	case messageTypeSubscribeDone:
		return "SubscribeDone"

	case messageTypePublish:
		return "Publish"
	case messageTypePublishOk:
		return "PublishOk"
	case messageTypePublishError:
		return "PublishError"

	case messageTypeFetch:
		return "Fetch"
	case messageTypeFetchOk:
		return "FetchOk"
	case messageTypeFetchError:
		return "FetchError"
	case messageTypeFetchCancel:
		return "FetchCancel"

	case messageTypeTrackStatus:
		return "TrackStatus"
	case messageTypeTrackStatusOk:
		return "TrackStatusOk"
	case messageTypeTrackStatusError:
		return "TrackStatusError"

	case messageTypeAnnounce:
		return "Announce"
	case messageTypeAnnounceOk:
		return "AnnounceOk"
	case messageTypeAnnounceError:
		return "AnnounceError"
	case messageTypeUnannounce:
		return "Unannounce"
	case messageTypeAnnounceCancel:
		return "AnnounceCancel"

	case messageTypeSubscribeNamespace:
		return "SubscribeNamespace"
	case messageTypeSubscribeNamespaceOk:
		return "SubscribeNamespaceOk"
	case messageTypeSubscribeNamespaceError:
		return "SubscribeNamespaceError"
	case messageTypeUnsubscribeNamespace:
		return "UnsubscribeNamespace"
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
