package wire

import (
	"bufio"
	"fmt"
	"io"

	"github.com/quic-go/quic-go/quicvarint"
)

type ObjectHeaderExtension interface {
	append([]byte) []byte
	parse([]byte) (int, error)
	parseReader(*bufio.Reader) error
	key() uint64
	String() string
}

type ObjectHeaderExtensions []ObjectHeaderExtension

func (ee ObjectHeaderExtensions) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(len(ee)))
	for _, e := range ee {
		buf = e.append(buf)
	}
	return buf
}

func (ee ObjectHeaderExtensions) String() string {
	res := "["
	for i, e := range ee {
		if i < len(ee)-1 {
			res += fmt.Sprintf("%v, ", e)
		} else {
			res += fmt.Sprintf("%v", e)
		}
	}
	return res + "]"
}

func (ee ObjectHeaderExtensions) parse(data []byte) (parsed int, err error) {
	num, n, err := quicvarint.Parse(data)
	parsed += n
	if err != nil {
		return
	}
	data = data[n:]

	for i := uint64(0); i < num; i++ {
		var t uint64
		t, n, err = quicvarint.Parse(data)
		parsed += n
		if err != nil {
			return
		}
		data = data[n:]
		var hdrExt ObjectHeaderExtension
		if t%2 == 0 {
			hdrExt = &VarintObjectExtensionHeader{}
		} else {
			hdrExt = &TLVObjectExtensionHeader{}
		}
		n, err = hdrExt.parse(data)
		parsed += n
		if err != nil {
			return
		}
		data = data[n:]
		ee = append(ee, hdrExt)
	}
	return
}

func (ee ObjectHeaderExtensions) parseReader(br *bufio.Reader) error {
	num, err := quicvarint.Read(br)
	if err != nil {
		return err
	}
	for i := uint64(0); i < num; i++ {
		t, err := quicvarint.Read(br)
		if err != nil {
			return err
		}
		var hdrExt ObjectHeaderExtension
		if t%2 == 0 {
			hdrExt = &VarintObjectExtensionHeader{}
		} else {
			hdrExt = &TLVObjectExtensionHeader{}
		}
		if err = hdrExt.parseReader(br); err != nil {
			return err
		}
		ee = append(ee, hdrExt)
	}
	return nil
}

type VarintObjectExtensionHeader struct {
	Type  uint64
	Value uint64
}

// String implements ObjectHeaderExtension.
func (h VarintObjectExtensionHeader) String() string {
	return fmt.Sprintf("{type: %v, value: %v}", h.Type, h.Value)
}

// append implements ObjectHeaderExtension.
func (h VarintObjectExtensionHeader) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, h.Type)
	buf = quicvarint.Append(buf, h.Value)
	return buf
}

// key implements ObjectHeaderExtension.
func (h VarintObjectExtensionHeader) key() uint64 {
	return h.Type
}

func (h *VarintObjectExtensionHeader) parse(data []byte) (parsed int, err error) {
	h.Value, parsed, err = quicvarint.Parse(data)
	return
}

func (h *VarintObjectExtensionHeader) parseReader(r *bufio.Reader) (err error) {
	h.Value, err = quicvarint.Read(r)
	return
}

type TLVObjectExtensionHeader struct {
	Type  uint64
	Value []byte
}

// String implements ObjectHeaderExtension.
func (t *TLVObjectExtensionHeader) String() string {
	return fmt.Sprintf("{type: %v, value: %v}", t.Type, t.Value)
}

// append implements ObjectHeaderExtension.
func (t *TLVObjectExtensionHeader) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, t.Type)
	buf = appendVarIntBytes(buf, t.Value)
	return buf
}

// key implements ObjectHeaderExtension.
func (t *TLVObjectExtensionHeader) key() uint64 {
	return t.Type
}

// parse implements ObjectHeaderExtension.
func (t *TLVObjectExtensionHeader) parse(data []byte) (n int, err error) {
	t.Value, n, err = parseVarIntBytes(data)
	return
}

// parse implements ObjectHeaderExtension.
func (t *TLVObjectExtensionHeader) parseReader(br *bufio.Reader) (err error) {
	length, err := quicvarint.Read(br)
	if err != nil {
		return err
	}
	t.Value = make([]byte, length)
	n, err := io.ReadFull(br, t.Value)
	if err != nil {
		return err
	}
	if uint64(n) != length {
		return errLengthMismatch
	}
	return nil
}
