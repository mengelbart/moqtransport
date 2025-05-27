# Media over QUIC Transport (MoQT)

[![Go Reference](https://pkg.go.dev/badge/github.com/mengelbart/moqtransport.svg)](https://pkg.go.dev/github.com/mengelbart/moqtransport)

`moqtransport` is a Go implementation of [Media over QUIC Transport](https://datatracker.ietf.org/doc/draft-ietf-moq-transport/) on top of [quic-go](https://github.com/quic-go/quic-go) and optionally [webtransport-go](https://github.com/quic-go/webtransport-go/).

## Overview

This library implements the Media over QUIC Transport (MoQT) protocol as defined in [draft-ietf-moq-transport-11](https://www.ietf.org/archive/id/draft-ietf-moq-transport-11.txt). MoQT is designed to operate over QUIC or WebTransport for efficient media delivery with a publish/subscribe model.

### Implementation Status

This code, as well as the specification, is work in progress.
The implementation currently covers most aspects of the MoQT specification (draft-11), including:

 Session establishment and initialization  
 Control message encoding and handling  
 Data stream management  
 Track announcement and subscription  
 Error handling  
 Support for both QUIC and WebTransport  

### Areas for Future Development

 Implementation of FETCH
 Exposure of more parameters
 ...

## Usage

See the [date examples in the examples directory](examples/date/README.md) for a simple demonstration of how to use this library.

Basic usage involves:

1. Creating a connection using either QUIC or WebTransport
2. Establishing a MoQT session
3. Implementing handlers for various MoQT messages
4. Publishing or subscribing to tracks

## Project Structure

- `quicmoq/`: QUIC-specific implementation
- `webtransportmoq/`: WebTransport-specific implementation
- `internal/`: Internal implementation details
- `examples/`: Example applications demonstrating usage
- `integrationtests/`: Integration tests

## Requirements

- Go 1.23.6 or later
- Dependencies are managed via Go modules

## License

See the [LICENSE](LICENSE) file for details.
