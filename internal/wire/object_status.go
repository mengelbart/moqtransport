package wire

type ObjectStatus int

const (
	ObjectStatusNormal             ObjectStatus = 0x00
	ObjectStatusObjectDoesNotExist ObjectStatus = 0x01
	ObjectStatusEndOfGroup         ObjectStatus = 0x03
	ObjectStatusEndOfTrack         ObjectStatus = 0x04
)
