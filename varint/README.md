# varint

This package is forked from
[quic-go](https://github.com/quic-go/quic-go/tree/v0.39.0/quicvarint). The fork
adds the `ReadWithLen(r io.ByteReader) (uint64, int, error)` function to return
the number of bytes read from a reader, which is used in the MoQ Transport
parser.

