package wire

import "io"

type SubgroupObject struct {
	hasProperties bool

	// Payload carries the object payload when the object is written.
	Payload []byte
	// PayloadReader carries the object payload when the object is parsed. It
	// is valid until the next read from the parser it came from.
	PayloadReader io.Reader

	ObjectIDDelta uint64         `proto:"varint"`
	Properties    []KeyValuePair `proto:"kvp_list_tlv,if=HasProperties"`
	PayloadLength uint64         `proto:"varint"`
	ObjectStatus  uint64         `proto:"varint,if=EmptyPayload"`
}

func (m *SubgroupObject) Type() ControlMessageType {
	return 0
}

func (m *SubgroupObject) HasProperties() bool {
	return m.hasProperties
}

func (m *SubgroupObject) SetHasProperties(v bool) {
	m.hasProperties = v
}

func (m *SubgroupObject) EmptyPayload() bool {
	return m.PayloadLength == 0
}
