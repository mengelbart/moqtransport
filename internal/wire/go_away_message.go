package wire

import (
	"log/slog"

	"github.com/mengelbart/qlog"
)

type GoAwayMessage struct {
	NewSessionURI string
}

func (m *GoAwayMessage) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("type", "goaway"),
		slog.Any("new_session_uri", qlog.RawInfo{
			Length:        uint64(len(m.NewSessionURI)),
			PayloadLength: uint64(len(m.NewSessionURI)),
			Data:          []byte(m.NewSessionURI),
		}),
	)
}

func (m GoAwayMessage) Type() controlMessageType {
	return messageTypeGoAway
}

func (m *GoAwayMessage) Append(buf []byte) []byte {
	buf = appendVarIntBytes(buf, []byte(m.NewSessionURI))
	return buf
}

func (m *GoAwayMessage) parse(_ Version, data []byte) (err error) {
	newSessionURI, _, err := parseVarIntBytes(data)
	m.NewSessionURI = string(newSessionURI)
	return err
}
