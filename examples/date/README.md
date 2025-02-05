# Example: Date server and client

The `examples` directory contains an implementation of a simple client and
server which can publish and subscribe to `date` tracks using MoQ. The publisher
of a `date` track publishes timestamps every second and sends them to the
subscribers. Subscribers receive the timestamp objects.

Download the example:

```shell
git clone https://github.com/mengelbart/moqtransport.git
cd moqtransport/examples/date
```

To run the server, run:

```shell
go run *.go -server -publish
```

Then, open a new shell and start a client:

```shell
go run *.go -subscribe
```

To use WebTransport, add the `-webtransport` and `-addr` flags:

Server:

```shell
go run *.go -server -publish -webtransport
```

Client:

```shell
go run *.go -subscribe -webtransport -addr https://localhost:8080/moq
```

