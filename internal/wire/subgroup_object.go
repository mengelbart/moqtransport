package wire

type SubgroupObject struct {
	hasProperties bool

	ObjectIDDelta uint64         `proto:"varint"`
	Properties    []KeyValuePair `proto:"kvp_list_tlv,if=HasProperties"`
	ObjectPayload []byte         `proto:"tlv_bytes"`
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
	return len(m.ObjectPayload) == 0
}
