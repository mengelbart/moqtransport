// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/mengelbart/moqtransport (interfaces: ControlMessageParser)
//
// Generated by this command:
//
//	mockgen -build_flags=-tags=gomock -typed -package moqtransport -write_package_comment=false -self_package github.com/mengelbart/moqtransport -destination mock_control_message_parser_test.go github.com/mengelbart/moqtransport ControlMessageParser
//

package moqtransport

import (
	reflect "reflect"

	wire "github.com/mengelbart/moqtransport/internal/wire"
	gomock "go.uber.org/mock/gomock"
)

// MockControlMessageParser is a mock of ControlMessageParser interface.
type MockControlMessageParser struct {
	ctrl     *gomock.Controller
	recorder *MockControlMessageParserMockRecorder
	isgomock struct{}
}

// MockControlMessageParserMockRecorder is the mock recorder for MockControlMessageParser.
type MockControlMessageParserMockRecorder struct {
	mock *MockControlMessageParser
}

// NewMockControlMessageParser creates a new mock instance.
func NewMockControlMessageParser(ctrl *gomock.Controller) *MockControlMessageParser {
	mock := &MockControlMessageParser{ctrl: ctrl}
	mock.recorder = &MockControlMessageParserMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockControlMessageParser) EXPECT() *MockControlMessageParserMockRecorder {
	return m.recorder
}

// Parse mocks base method.
func (m *MockControlMessageParser) Parse() (wire.ControlMessage, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Parse")
	ret0, _ := ret[0].(wire.ControlMessage)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Parse indicates an expected call of Parse.
func (mr *MockControlMessageParserMockRecorder) Parse() *MockControlMessageParserParseCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Parse", reflect.TypeOf((*MockControlMessageParser)(nil).Parse))
	return &MockControlMessageParserParseCall{Call: call}
}

// MockControlMessageParserParseCall wrap *gomock.Call
type MockControlMessageParserParseCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockControlMessageParserParseCall) Return(arg0 wire.ControlMessage, arg1 error) *MockControlMessageParserParseCall {
	c.Call = c.Call.Return(arg0, arg1)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockControlMessageParserParseCall) Do(f func() (wire.ControlMessage, error)) *MockControlMessageParserParseCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockControlMessageParserParseCall) DoAndReturn(f func() (wire.ControlMessage, error)) *MockControlMessageParserParseCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}
