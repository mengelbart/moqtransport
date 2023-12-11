//go:build js && wasm
// +build js,wasm

package moqtransport

func ClientSetupMessage(role Role) *clientSetupMessage {
	return &clientSetupMessage{
		SupportedVersions: []version{DRAFT_IETF_MOQ_TRANSPORT_01},
		SetupParameters: map[uint64]parameter{
			roleParameterKey: varintParameter{
				k: roleParameterKey,
				v: uint64(role),
			},
		},
	}
}

func (c *clientSetupMessage) Bytes() []byte {
	return c.append(make([]byte, 0, 1500))
}
