// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/mengelbart/moqtransport (interfaces: ReceiveStream)
//
// Generated by this command:
//
//	mockgen -build_flags=-tags=gomock -package moqtransport -self_package github.com/mengelbart/moqtransport -destination mock_receive_stream_test.go github.com/mengelbart/moqtransport ReceiveStream
//

// Package moqtransport is a generated GoMock package.
package moqtransport

import (
	reflect "reflect"

	gomock "go.uber.org/mock/gomock"
)

// MockReceiveStream is a mock of ReceiveStream interface.
type MockReceiveStream struct {
	ctrl     *gomock.Controller
	recorder *MockReceiveStreamMockRecorder
	isgomock struct{}
}

// MockReceiveStreamMockRecorder is the mock recorder for MockReceiveStream.
type MockReceiveStreamMockRecorder struct {
	mock *MockReceiveStream
}

// NewMockReceiveStream creates a new mock instance.
func NewMockReceiveStream(ctrl *gomock.Controller) *MockReceiveStream {
	mock := &MockReceiveStream{ctrl: ctrl}
	mock.recorder = &MockReceiveStreamMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockReceiveStream) EXPECT() *MockReceiveStreamMockRecorder {
	return m.recorder
}

// Read mocks base method.
func (m *MockReceiveStream) Read(p []byte) (int, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Read", p)
	ret0, _ := ret[0].(int)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Read indicates an expected call of Read.
func (mr *MockReceiveStreamMockRecorder) Read(p any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Read", reflect.TypeOf((*MockReceiveStream)(nil).Read), p)
}
