package wire

import "github.com/mengelbart/moqtransport/varint"

func (m *FetchHeader) parse(r streamReader) error {
	var err error
	m.RequestID, err = varint.Read(r)
	return err
}
