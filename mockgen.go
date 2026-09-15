package moqtransport

//go:generate go tool mockgen -typed -write_package_comment=false -package moqtransport -destination mock_connection_test.go github.com/mengelbart/moqtransport/quic Connection
//go:generate go tool mockgen -typed -write_package_comment=false -package moqtransport -destination mock_stream_test.go github.com/mengelbart/moqtransport/quic Stream
//go:generate go tool mockgen -typed -write_package_comment=false -package moqtransport -destination mock_receive_stream_test.go github.com/mengelbart/moqtransport/quic ReceiveStream
//go:generate go tool mockgen -typed -write_package_comment=false -package moqtransport -destination mock_send_stream_test.go github.com/mengelbart/moqtransport/quic SendStream
//go:generate go tool mockgen -typed -write_package_comment=false -package moqtransport -self_package github.com/mengelbart/moqtransport -destination mock_handler_test.go github.com/mengelbart/moqtransport Handler
