package wire

type ObjectStream struct {
	hasProperties bool

	ObjectIDDelta uint64         `proto:"varint"`
	Properties    []KeyValuePair `proto:"tlv_message_list,if=HasProperties"`
	ObjectPayload []byte         `proto:"tlv_bytes"`
	ObjectStatus  uint64         `proto:"varint,if=EmptyPayload"`
}

func (m *ObjectStream) Type() ControlMessageType {
	return 0
}

func (m *ObjectStream) HasProperties() bool {
	return m.hasProperties
}

func (m *ObjectStream) SetHasProperties(v bool) {
	m.hasProperties = v
}

func (m *ObjectStream) EmptyPayload() bool {
	return len(m.ObjectPayload) == 0
}
