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
	buf = varint.Append(buf, uint64(msg.Type()))
	tl := len(buf)
	buf = append(buf, 0x00, 0x00) // length placeholder

	switch a.version {
	case 18:
		buf = msg.append_v18(buf)
	default:
		return fmt.Errorf("unsupported version: %d", a.version)
	}

	length := len(buf[tl+2:])
	if length > math.MaxUint16 {
		return fmt.Errorf("control message too large")
	}
	binary.BigEndian.PutUint16(buf[tl:tl+2], uint16(length))

	n, err := a.writer.Write(buf)
	if err != nil {
		return err
	}
	if n != len(buf) {
		return fmt.Errorf("failed to write complete message")
	}
	return nil
}
