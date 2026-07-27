package wire2

import (
	"fmt"
	"io"
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
	buf := make([]byte, 0, 1024)
	switch a.version {
	case 18:
		buf = msg.append_v18(buf)
	default:
		return fmt.Errorf("unsupported version: %d", a.version)
	}
	n, err := a.writer.Write(buf)
	if err != nil {
		return err
	}
	if n != len(buf) {
		return fmt.Errorf("failed to write complete message")
	}
	return nil
}
