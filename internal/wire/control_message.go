package wire

const (
	// Bidirectional stream messages
	ControlMessageTypeSubscribe   ControlMessageType = 0x3
	ControlMessageTypeSubscribeOk ControlMessageType = 0x4

	ControlMessageTypePublish     ControlMessageType = 0x1d
	ControlMessageTypePublishOk   ControlMessageType = 0x1e
	ControlMessageTypePublishDone ControlMessageType = 0xb

	ControlMessageTypeFetch   ControlMessageType = 0x16
	ControlMessageTypeFetchOk ControlMessageType = 0x18

	ControlMessageTypeTrackStatus ControlMessageType = 0xd

	ControlMessageTypePublishNamespace   ControlMessageType = 0x6
	ControlMessageTypeSubscribeNamespace ControlMessageType = 0x50
	ControlMessageTypeSubscribeTracks    ControlMessageType = 0x51

	ControlMessageTypeNamespace     ControlMessageType = 0x8
	ControlMessageTypeNamespaceDone ControlMessageType = 0xe

	ControlMessageTypePublishBlocked ControlMessageType = 0xf

	ControlMessageTypeRequestUpdate ControlMessageType = 0x2
	ControlMessageTypeRequestOk     ControlMessageType = 0x7
	ControlMessageTypeRequestError  ControlMessageType = 0x5

	// Unidirectional stream messages
	ControlMessageTypeSetup ControlMessageType = 0x2f00

	ControlMessageTypeGoAway ControlMessageType = 0x10

	ControlMessageTypeFetchHeader ControlMessageType = 0x5
	ControlMessageTypePadding     ControlMessageType = 0x132b3e28
)

type ControlMessageType uint64

type ControlMessage interface {
	Type() ControlMessageType
	parse_v18(messageReader) error
	append_v18([]byte) []byte
}
