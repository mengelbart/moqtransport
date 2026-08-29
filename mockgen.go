package moqtransport

//go:generate go tool mockgen -typed -write_package_comment=false -package moqtransport -self_package github.com/mengelbart/moqtransport -destination mock_connection_test.go github.com/mengelbart/moqtransport Connection
//go:generate go tool mockgen -typed -write_package_comment=false -package moqtransport -self_package github.com/mengelbart/moqtransport -destination mock_stream_test.go github.com/mengelbart/moqtransport Stream
//go:generate go tool mockgen -typed -write_package_comment=false -package moqtransport -self_package github.com/mengelbart/moqtransport -destination mock_receive_stream_test.go github.com/mengelbart/moqtransport ReceiveStream
//go:generate go tool mockgen -typed -write_package_comment=false -package moqtransport -self_package github.com/mengelbart/moqtransport -destination mock_send_stream_test.go github.com/mengelbart/moqtransport SendStream
//go:generate go tool mockgen -typed -write_package_comment=false -package moqtransport -self_package github.com/mengelbart/moqtransport -destination mock_handler_test.go github.com/mengelbart/moqtransport Handler
