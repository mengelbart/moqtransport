package wire2

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"github.com/mengelbart/moqtransport/varint"
)

type Appender struct {
	writer  io.Writer
	version uint64
}

func NewAppender(w io.Writer, version uint64) *Appender {
	return &Appender{
		writer:  w,
		version: version,
	}
}

func (a *Appender) Write(msg ControlMessage) error {
	buf := make([]byte, 0, 4096)

	if _, ok := msg.(*ObjectStream); ok {
		// ObjectStream messages are written without type and length.
		buf, err := a.serializeMessage(buf, msg)
		if err != nil {
			return err
		}
		return a.writeBuffer(buf)
	}

	buf = varint.Append(buf, uint64(msg.Type()))
	tl := len(buf)
	buf = append(buf, 0x00, 0x00) // length placeholder

	buf, err := a.serializeMessage(buf, msg)
	if err != nil {
		return err
	}

	length := len(buf[tl+2:])
	if length > math.MaxUint16 {
		return fmt.Errorf("control message too large")
	}
	binary.BigEndian.PutUint16(buf[tl:tl+2], uint16(length))

	return a.writeBuffer(buf)
}

func (a *Appender) serializeMessage(buf []byte, msg ControlMessage) ([]byte, error) {
	switch a.version {
	case 18:
		buf = msg.append_v18(buf)
	default:
		return nil, fmt.Errorf("unsupported version: %d", a.version)
	}
	return buf, nil
}

func (a *Appender) writeBuffer(buf []byte) error {
	n, err := a.writer.Write(buf)
	if err != nil {
		return err
	}
	if n != len(buf) {
		return fmt.Errorf("failed to write complete message")
	}
	return nil
}
