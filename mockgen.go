//go:build gomock || generate

package moqtransport

import "github.com/mengelbart/moqtransport/internal/wire"

//go:generate sh -c "go run go.uber.org/mock/mockgen -build_flags=\"-tags=gomock\" -typed -package moqtransport -write_package_comment=false -self_package github.com/mengelbart/moqtransport -destination mock_stream_test.go github.com/mengelbart/moqtransport Stream"

//go:generate sh -c "go run go.uber.org/mock/mockgen -build_flags=\"-tags=gomock\" -typed -package moqtransport -write_package_comment=false -self_package github.com/mengelbart/moqtransport -destination mock_receive_stream_test.go github.com/mengelbart/moqtransport ReceiveStream"

//go:generate sh -c "go run go.uber.org/mock/mockgen -build_flags=\"-tags=gomock\" -typed -package moqtransport -write_package_comment=false -self_package github.com/mengelbart/moqtransport -destination mock_send_stream_test.go github.com/mengelbart/moqtransport SendStream"

//go:generate sh -c "go run go.uber.org/mock/mockgen -build_flags=\"-tags=gomock\" -typed -package moqtransport -write_package_comment=false -self_package github.com/mengelbart/moqtransport -destination mock_connection_test.go github.com/mengelbart/moqtransport Connection"

//go:generate sh -c "go run go.uber.org/mock/mockgen -build_flags=\"-tags=gomock\" -typed -package moqtransport -write_package_comment=false -self_package github.com/mengelbart/moqtransport -destination mock_control_message_parser_test.go github.com/mengelbart/moqtransport ControlMessageParser"
type ControlMessageParser = controlMessageParser

//go:generate sh -c "go run go.uber.org/mock/mockgen -build_flags=\"-tags=gomock\" -typed -package moqtransport -write_package_comment=false -self_package github.com/mengelbart/moqtransport -destination mock_control_message_recv_queue_test.go github.com/mengelbart/moqtransport ControlMessageRecvQueue"
type ControlMessageRecvQueue = controlMessageQueue[Message]

//go:generate sh -c "go run go.uber.org/mock/mockgen -build_flags=\"-tags=gomock\" -typed -package moqtransport -write_package_comment=false -self_package github.com/mengelbart/moqtransport -destination mock_control_message_send_test.go github.com/mengelbart/moqtransport ControlMessageSendQueue"
type ControlMessageSendQueue = controlMessageQueue[wire.ControlMessage]

//go:generate sh -c "go run go.uber.org/mock/mockgen -build_flags=\"-tags=gomock\" -typed -package moqtransport -write_package_comment=false -self_package github.com/mengelbart/moqtransport -destination mock_object_message_parser_test.go github.com/mengelbart/moqtransport ObjectMessageParser"
type ObjectMessageParser = objectMessageParser
