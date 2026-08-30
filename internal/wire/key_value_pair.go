package wire

type KeyValuePair struct {
	Type   uint64 `proto:"varint"`
	Bytes  []byte `proto:"tlv_bytes,if=hasBytes"`
	Varint uint64 `proto:"varint,if=!hasBytes"`
}

// hasBytes reports whether the pair carries a byte string rather than a varint.
func (p *KeyValuePair) hasBytes() bool {
	return p.Type%2 == 1
}
