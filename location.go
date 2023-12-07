package moqtransport

import "github.com/quic-go/quic-go/quicvarint"

type LocationMode int

const (
	LocationModeNone LocationMode = iota
	LocationModeAbsolute
	LocationModeRelativePrevious
	LocationModeRelativeNext
)

type Location struct {
	Mode  LocationMode
	Value uint64
}

func (l Location) append(buf []byte) []byte {
	if l.Mode == 0 {
		return append(buf, byte(l.Mode))
	}
	buf = append(buf, byte(l.Mode))
	return append(buf, byte(l.Value))
}

func (p *loggingParser) parseLocation() (Location, error) {
	if p.reader == nil {
		return Location{}, errInvalidMessageReader
	}
	mode, err := quicvarint.Read(p.reader)
	if err != nil {
		return Location{}, err
	}
	if LocationMode(mode) == LocationModeNone {
		return Location{
			Mode:  LocationMode(mode),
			Value: 0,
		}, nil
	}
	value, err := quicvarint.Read(p.reader)
	if err != nil {
		return Location{}, nil
	}
	return Location{
		Mode:  LocationMode(mode),
		Value: value,
	}, nil
}
