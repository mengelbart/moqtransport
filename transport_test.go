package moqtransport

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/mengelbart/moqtransport/internal/wire"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func mockSessionFactory(s *MockSessionI) sessionFactory {
	return func(_ sessionCallbacks, _ Perspective, _ Protocol, _ ...sessionOption) (sessionI, error) {
		return s, nil
	}
}

func mockControlMessageParser(p *MockControlMessageParserI) controlMessageParserFactory {
	return func(_ io.Reader) controlMessageParserI {
		return p
	}
}

func TestNewTransport(t *testing.T) {
	protos := []Protocol{ProtocolQUIC, ProtocolWebTransport}
	perspectives := []Perspective{PerspectiveServer, PerspectiveClient}

	for _, proto := range protos {
		for _, perspective := range perspectives {
			t.Run(fmt.Sprintf("%v_%v", proto, perspective), func(t *testing.T) {
				ctrl := gomock.NewController(t)
				mc := NewMockConnection(ctrl)
				ms := NewMockSessionI(ctrl)
				mst := NewMockStream(ctrl)
				cmp := NewMockControlMessageParserI(ctrl)

				mc.EXPECT().Protocol().Return(proto).AnyTimes()
				mc.EXPECT().Perspective().Return(perspective).AnyTimes()

				closeControlStream := make(chan struct{})
				closeConnection := make(chan struct{})

				switch perspective {
				case PerspectiveServer:
					mc.EXPECT().AcceptStream(gomock.Any()).Return(mst, nil)
					cmp.EXPECT().Parse().Return(&wire.ClientSetupMessage{}, nil)
					ms.EXPECT().onControlMessage(&wire.ClientSetupMessage{}).Return(nil)
				case PerspectiveClient:
					mc.EXPECT().OpenStreamSync(gomock.Any()).Return(mst, nil)
					ms.EXPECT().sendClientSetup().Return(nil)
					cmp.EXPECT().Parse().Return(&wire.ServerSetupMessage{}, nil)
					ms.EXPECT().onControlMessage(&wire.ServerSetupMessage{}).Return(nil)
				}
				ms.EXPECT().getSetupDone().Return(true)
				mc.EXPECT().CloseWithError(gomock.Any(), gomock.Any()).DoAndReturn(func(uint64, string) error {
					close(closeControlStream)
					close(closeConnection)
					return nil
				})
				cmp.EXPECT().Parse().DoAndReturn(func() (wire.ControlMessage, error) {
					<-closeControlStream
					return nil, io.EOF
				})
				mc.EXPECT().AcceptUniStream(gomock.Any()).DoAndReturn(func(context.Context) (Stream, error) {
					<-closeConnection
					return nil, io.EOF
				})
				mc.EXPECT().ReceiveDatagram(gomock.Any()).DoAndReturn(func(context.Context) ([]byte, error) {
					<-closeConnection
					return nil, io.EOF
				})

				transport, err := NewTransport(
					mc,
					setSessionFactory(mockSessionFactory(ms)),
					setControlMessageParserFactory(mockControlMessageParser(cmp)),
				)
				assert.NoError(t, err)
				assert.NotNil(t, transport)
				select {
				case <-transport.handshakeDone:
				case <-time.After(time.Second):
					assert.Fail(t, "handshake timeout")
				}

				assert.NoError(t, transport.Close())
			})
		}
	}
}

func TestTransportAnnounce(t *testing.T) {
	t.Skip()
	getTransport := func(t *testing.T) (*MockSessionI, *Transport) {
		ctrl := gomock.NewController(t)
		ms := NewMockSessionI(ctrl)
		transport := &Transport{
			ctx:              nil,
			cancelCtx:        nil,
			wg:               sync.WaitGroup{},
			logger:           &slog.Logger{},
			conn:             nil,
			controlStream:    nil,
			ctrlMsgSendQueue: make(chan wire.ControlMessage),
			session:          ms,
			callbacks:        &callbacks{},
		}
		return ms, transport
	}
	t.Run("succesful_announcement", func(t *testing.T) {
		ms, transport := getTransport(t)
		ms.EXPECT().announce(gomock.Any()).DoAndReturn(func(a *announcement) error {
			assert.Equal(t, []string{"namespace"}, a.Namespace)
			assert.Equal(t, wire.Parameters{}, a.parameters)
			a.response <- nil
			return nil
		})

		err := transport.Announce(context.Background(), []string{"namespace"})
		assert.NoError(t, err)
	})
	t.Run("succesful_announcement", func(t *testing.T) {
		ms, transport := getTransport(t)
		ms.EXPECT().announce(gomock.Any()).DoAndReturn(func(a *announcement) error {
			assert.Equal(t, []string{"namespace"}, a.Namespace)
			assert.Equal(t, wire.Parameters{}, a.parameters)
			a.response <- ProtocolError{
				code:    ErrorCodeAnnouncementUnauthorized,
				message: "unauthorized",
			}
			return nil
		})

		err := transport.Announce(context.Background(), []string{"namespace"})
		assert.Error(t, err)
		assert.ErrorContains(t, err, "unauthorized")
	})
}
