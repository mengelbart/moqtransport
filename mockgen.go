//go:build gomock || generate

package moqtransport

//go:generate sh -c "go run go.uber.org/mock/mockgen -build_flags=\"-tags=gomock\" -package moqtransport -self_package github.com/mengelbart/moqtransport -destination mock_stream_test.go github.com/mengelbart/moqtransport Stream"

//go:generate sh -c "go run go.uber.org/mock/mockgen -build_flags=\"-tags=gomock\" -package moqtransport -self_package github.com/mengelbart/moqtransport -destination mock_receive_stream_test.go github.com/mengelbart/moqtransport ReceiveStream"

//go:generate sh -c "go run go.uber.org/mock/mockgen -build_flags=\"-tags=gomock\" -package moqtransport -self_package github.com/mengelbart/moqtransport -destination mock_send_stream_test.go github.com/mengelbart/moqtransport SendStream"

//go:generate sh -c "go run go.uber.org/mock/mockgen -build_flags=\"-tags=gomock\" -package moqtransport -self_package github.com/mengelbart/moqtransport -destination mock_connection_test.go github.com/mengelbart/moqtransport Connection"

//go:generate sh -c "go run go.uber.org/mock/mockgen -build_flags=\"-tags=gomock\" -package moqtransport -self_package github.com/mengelbart/moqtransport -destination mock_control_message_parser_test.go github.com/mengelbart/moqtransport ControlMessageParser"
type ControlMessageParser = controlMessageParser

//go:generate sh -c "go run go.uber.org/mock/mockgen -build_flags=\"-tags=gomock\" -package moqtransport -self_package github.com/mengelbart/moqtransport -destination mock_control_message_sender.go github.com/mengelbart/moqtransport ControlMessageSender"
type ControlMessageSender = controlMessageSender

//go:generate sh -c "go run go.uber.org/mock/mockgen -build_flags=\"-tags=gomock\" -package moqtransport -self_package github.com/mengelbart/moqtransport -destination mock_message_handler.go github.com/mengelbart/moqtransport MessageHandler"
type MessageHandler = messageHandler
