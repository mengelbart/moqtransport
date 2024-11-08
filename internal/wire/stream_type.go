package wire

type StreamType int

const (
	StreamTypeSubgroup StreamType = 0x04
	StreamTypeFetch    StreamType = 0x05
)
