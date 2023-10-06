# Media over QUIC Transport

`moqtransport` is an implementation of [Media over QUIC
Transport](https://datatracker.ietf.org/doc/draft-ietf-moq-transport/) on top of
[quic-go](https://github.com/quic-go/quic-go) and optionally
[webtransport-go](https://github.com/quic-go/webtransport-go/).

## Example: Chat Server and Client

The `examples` directory contains an implementation of [MoQ
Chat](https://afrind.github.io/draft-frindell-moq-chat/draft-frindell-moq-chat.html).

To run a simple chat server using MoQ Tranpsort on top of QUIC, run:

```shell
go run examples/server/main.go
```

Then, open a new shell and start a client:

```shell
go run examples/client/main.go
```

The client is a simple interactive shell that reads and writes messages from
`stdin` and `stdout` (input and output is currently not well synchronized).

Open another shell and run a second client to chat with the first one using the
commands `join <roomID> <username>` to join a room and `msg <roomID> <message>`
to send messages to a room.

To use WebTransport, we need to create a TLS certificate. This can be done using
[mkcert](https://github.com/FiloSottile/mkcert):

```shell
mkcert localhost
mkcert -install
```

`mkcert` will generate a `localhost.pem` and a `localhost-key.pem` file. If you
change the name of the files or use a different host name, you can use the
`-cert` and `-key` flags of the server command to point it to the correct files.

We can now start the server and client with the `-webtransport` flag to run MoQ
Transport on top of WebTransport.

