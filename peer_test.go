package moqtransport

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/quic-go/quic-go/quicvarint"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestNewServerPeer(t *testing.T) {
	t.Run("nil_connection", func(t *testing.T) {
		_, err := newServerPeer(nil, nil)
		assert.Error(t, err)
	})
	t.Run("run_handshake", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ctrl := gomock.NewController(t)
		mc := NewMockConnection(ctrl)
		ms := NewMockStream(ctrl)
		mpf := NewMockParserFactory(ctrl)
		mp := NewMockParser(ctrl)
		mpf.EXPECT().new(gomock.Any()).AnyTimes().Return(mp)
		mc.EXPECT().AcceptStream(gomock.Any()).Times(1).Return(ms, nil)
		mp.EXPECT().parse().Times(1).Return(&clientSetupMessage{
			SupportedVersions: []version{DRAFT_IETF_MOQ_TRANSPORT_01},
			SetupParameters: map[uint64]parameter{
				roleParameterKey: varintParameter{
					k: roleParameterKey,
					v: uint64(IngestionDeliveryRole),
				},
			},
		}, nil)
		ms.EXPECT().Write(gomock.Any()).DoAndReturn(func(b []byte) (int, error) {
			msg := parse(t, b)
			assert.Equal(t, &serverSetupMessage{
				SelectedVersion: DRAFT_IETF_MOQ_TRANSPORT_01,
				SetupParameters: map[uint64]parameter{},
			}, msg)
			return len(b), nil
		})
		mp.EXPECT().parse().AnyTimes().DoAndReturn(func() (message, error) {
			<-ctx.Done()
			return nil, errors.New("connection closed")
		})

		p, err := newServerPeer(mc, mpf)
		assert.NoError(t, err)
		assert.NotNil(t, p)
	})
}

func TestNewClientPeer(t *testing.T) {
	t.Run("nil_connection", func(t *testing.T) {
		_, err := newClientPeer(nil, 0, nil)
		assert.Error(t, err)
	})
	t.Run("run_handshake", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ctrl := gomock.NewController(t)
		mc := NewMockConnection(ctrl)
		ms := NewMockStream(ctrl)
		mp := NewMockParser(ctrl)
		mpf := NewMockParserFactory(ctrl)
		mpf.EXPECT().new(gomock.Any()).Return(mp)

		mc.EXPECT().OpenStreamSync(gomock.Any()).Times(1).Return(ms, nil)
		ms.EXPECT().Write(gomock.Any()).DoAndReturn(func(b []byte) (int, error) {
			msg := parse(t, b)
			assert.Equal(t, &clientSetupMessage{
				SupportedVersions: []version{DRAFT_IETF_MOQ_TRANSPORT_01},
				SetupParameters: map[uint64]parameter{
					roleParameterKey: varintParameter{
						k: roleParameterKey,
						v: uint64(IngestionDeliveryRole),
					},
				},
			}, msg)
			return len(b), nil
		})
		mp.EXPECT().parse().Times(1).Return(&serverSetupMessage{
			SelectedVersion: DRAFT_IETF_MOQ_TRANSPORT_01,
			SetupParameters: map[uint64]parameter{},
		}, nil)
		mp = NewMockParser(ctrl)
		mpf.EXPECT().new(gomock.Any()).AnyTimes().Return(mp)
		mp.EXPECT().parse().AnyTimes().DoAndReturn(func() (message, error) {
			<-ctx.Done()
			return nil, errors.New("connection closed")
		})
		p, err := newClientPeer(mc, IngestionDeliveryRole, mpf)
		assert.NoError(t, err)
		assert.NotNil(t, p)
	})
}

func TestPeer(t *testing.T) {
	type env struct {
		ctrl *gomock.Controller
		peer *Peer
		mc   *MockConnection
	}
	setup := func(t *testing.T) (*env, func()) {
		ctx, cancel := context.WithCancel(context.Background())
		ctrl := gomock.NewController(t)
		mc := NewMockConnection(ctrl)
		mc.EXPECT().AcceptUniStream(gomock.Any()).AnyTimes().DoAndReturn(func(_ context.Context) (stream, error) {
			<-ctx.Done()
			return nil, errors.New("connection closed")
		})
		mCtrlStream := NewMockStream(ctrl)
		peer := &Peer{
			ctx:                   ctx,
			ctxCancel:             cancel,
			conn:                  mc,
			parserFactory:         nil,
			outgoingTransactionCh: make(chan keyedResponseHandler),
			outgoingCtrlMessageCh: make(chan message),
			incomingCtrlMessageCh: make(chan message),
			receiveTracks:         map[uint64]*ReceiveTrack{},
			sendTracks:            map[string]*SendTrack{},
			subscribeHandler:      nil,
			announcementHandler:   nil,
			closeCh:               make(chan struct{}),
			closeOnce:             sync.Once{},
			logger:                log.New(os.Stdout, "TEST_MOQ_PEER: ", log.LstdFlags),
		}
		go peer.controlLoop(mCtrlStream)
		go peer.Run(false)
		return &env{
				ctrl: ctrl,
				peer: peer,
				mc:   mc,
			}, func() {
				t.Log("TEARDOWN")
				assert.NoError(t, peer.Close())
			}
	}
	t.Run("subscribe", func(t *testing.T) {
		env, teardown := setup(t)
		defer teardown()
		go func() {
			msg := <-env.peer.outgoingCtrlMessageCh
			ctrlMsg, ok := msg.(*ctrlMessage)
			assert.True(t, ok)
			assert.Equal(t, &subscribeRequestMessage{
				TrackNamespace: "",
				TrackName:      "",
				StartGroup:     location{},
				StartObject:    location{},
				EndGroup:       location{},
				EndObject:      location{},
				Parameters:     map[uint64]parameter{},
			}, ctrlMsg.keyedMessage)
			env.peer.incomingCtrlMessageCh <- &subscribeOkMessage{
				TrackNamespace: "",
				TrackName:      "",
				TrackID:        0,
				Expires:        0,
			}
		}()
		rt, err := env.peer.Subscribe("", "", "")
		assert.NoError(t, err)
		assert.NotNil(t, rt)
	})
	t.Run("handle_subscribe", func(t *testing.T) {
		env, teardown := setup(t)
		defer teardown()
		ms := NewMockSendStream(env.ctrl)
		env.mc.EXPECT().OpenUniStream().Return(ms, nil)
		msh := NewMockSubscriptionHandler(env.ctrl)
		msh.EXPECT().handle(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(uint64(0), time.Duration(0), nil)
		env.peer.OnSubscription(msh)
		env.peer.incomingCtrlMessageCh <- &subscribeRequestMessage{
			TrackNamespace: "",
			TrackName:      "",
			StartGroup:     location{},
			StartObject:    location{},
			EndGroup:       location{},
			EndObject:      location{},
			Parameters:     map[uint64]parameter{},
		}
		res := <-env.peer.outgoingCtrlMessageCh
		assert.Equal(t, &subscribeOkMessage{
			TrackNamespace: "",
			TrackName:      "",
			TrackID:        0,
			Expires:        0,
		}, res)
	})
	t.Run("announce", func(t *testing.T) {
		env, teardown := setup(t)
		defer teardown()
		go func() {
			msg := <-env.peer.outgoingCtrlMessageCh
			ctrlMsg, ok := msg.(*ctrlMessage)
			assert.True(t, ok)
			assert.Equal(t, &announceMessage{
				TrackNamespace:         "namespace",
				TrackRequestParameters: map[uint64]parameter{},
			}, ctrlMsg.keyedMessage)
			env.peer.incomingCtrlMessageCh <- &announceOkMessage{
				TrackNamespace: "namespace",
			}
		}()
		err := env.peer.Announce("namespace")
		assert.NoError(t, err)
	})
	t.Run("announce_invalid_namespace", func(t *testing.T) {
		env, teardown := setup(t)
		defer teardown()
		err := env.peer.Announce("")
		assert.Error(t, err)
	})
}

// TODO: Manually parsing here is not great. Maybe we need another
// abstraction that we can mock to get the outgoing messages before
// they are serialized?
func parse(t *testing.T, buf []byte) message {
	parser := &loggingParser{
		logger: log.New(os.Stdout, "TEST_PARSER_LOG", log.LstdFlags),
		reader: quicvarint.NewReader(bytes.NewReader(buf)),
	}
	m, err := parser.parse()
	assert.NoError(t, err)
	return m
}
