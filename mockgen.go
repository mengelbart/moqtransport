//go:build gomock || generate

package moqtransport

//go:generate sh -c "go run go.uber.org/mock/mockgen -build_flags=\"-tags=gomock\" -typed -package moqtransport -write_package_comment=false -self_package github.com/mengelbart/moqtransport -destination mock_handler_test.go github.com/mengelbart/moqtransport Handler"

//go:generate sh -c "go run go.uber.org/mock/mockgen -build_flags=\"-tags=gomock\" -typed -package moqtransport -write_package_comment=false -self_package github.com/mengelbart/moqtransport -destination mock_stream_test.go github.com/mengelbart/moqtransport Stream"

//go:generate sh -c "go run go.uber.org/mock/mockgen -build_flags=\"-tags=gomock\" -typed -package moqtransport -write_package_comment=false -self_package github.com/mengelbart/moqtransport -destination mock_receive_stream_test.go github.com/mengelbart/moqtransport ReceiveStream"

//go:generate sh -c "go run go.uber.org/mock/mockgen -build_flags=\"-tags=gomock\" -typed -package moqtransport -write_package_comment=false -self_package github.com/mengelbart/moqtransport -destination mock_send_stream_test.go github.com/mengelbart/moqtransport SendStream"

//go:generate sh -c "go run go.uber.org/mock/mockgen -build_flags=\"-tags=gomock\" -typed -package moqtransport -write_package_comment=false -self_package github.com/mengelbart/moqtransport -destination mock_connection_test.go github.com/mengelbart/moqtransport Connection"

//go:generate sh -c "go run go.uber.org/mock/mockgen -build_flags=\"-tags=gomock\" -typed -package moqtransport -write_package_comment=false -self_package github.com/mengelbart/moqtransport -destination mock_object_message_parser_test.go github.com/mengelbart/moqtransport ObjectMessageParser"
type ObjectMessageParser = objectMessageParser

//go:generate sh -c "go run go.uber.org/mock/mockgen -build_flags=\"-tags=gomock\" -typed -package moqtransport -write_package_comment=false -self_package github.com/mengelbart/moqtransport -destination mock_control_message_stream_test.go github.com/mengelbart/moqtransport ControlMessageStream"
type ControlMessageStream = controlMessageStream
