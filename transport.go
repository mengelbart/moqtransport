package moqtransport

const (
	DRAFT_IETF_MOQ_TRANSPORT_00 = 0xff000000
	DRAFT_IETF_MOQ_TRANSPORT_01 = 0xff000001
)

const (
	ErrorCodeNoError           = 0x00
	ErrorCodeGeneric           = 0x01
	ErrorCodeUnauthorized      = 0x02
	ErrorCodeProtocolViolation = 0x03
	ErrorCodeGoAwayTimeout     = 0x10
)
