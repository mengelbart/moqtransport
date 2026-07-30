package moqtransport

const (
	MOQT18 ApplicationProtocol = "moqt-18"
)

type ApplicationProtocol string

func (ap ApplicationProtocol) String() string {
	return string(ap)
}

func (ap ApplicationProtocol) versionNumber() uint64 {
	switch ap {
	case MOQT18:
		return 18
	default:
		return 0
	}
}
