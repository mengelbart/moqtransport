package moqtransport

const (
	DRAFT_IETF_MOQ_TRANSPORT_00 = 0xff000000
	DRAFT_IETF_MOQ_TRANSPORT_01 = 0xff000001
)

const (
	SessionTerminatedErrorCode = 0x00
	GenericErrorCode           = 0x01
	UnauthorizedErrorCode      = 0x02
	GoAwayErrorCode            = 0x10
)
