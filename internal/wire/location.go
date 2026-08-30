package wire

type Location struct {
	Group  uint64 `proto:"varint"`
	Object uint64 `proto:"varint"`
}
