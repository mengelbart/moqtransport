package moqtransport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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
		mc.EXPECT().AcceptUniStream(gomock.Any()).AnyTimes().DoAndReturn(func(_ context.Context) (stream, error) {
			<-ctx.Done()
			return nil, errors.New("connection closed")
		})
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
		mc.EXPECT().CloseWithError(uint64(0), "")
		assert.NoError(t, p.CloseWithError(0, ""))
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
		mc.EXPECT().AcceptUniStream(gomock.Any()).AnyTimes().DoAndReturn(func(_ context.Context) (stream, error) {
			<-ctx.Done()
			return nil, errors.New("connection closed")
		})
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
		mc.EXPECT().CloseWithError(uint64(17), "")
		assert.NoError(t, p.CloseWithError(17, ""))
	})
}

func TestPeer(t *testing.T) {
	type env struct {
		ctrl      *gomock.Controller
		peer      *Peer
		ctrStream *MockStream
		mc        *MockConnection
	}
	setup := func(t *testing.T) (*env, func()) {
		ctx, cancel := context.WithCancelCause(context.Background())
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
			subscriptionCh:        make(chan *Subscription),
			announcementCh:        make(chan *Announcement),
			trackLock:             sync.RWMutex{},
			receiveTracks:         map[uint64]*ReceiveTrack{},
			sendTracks:            map[string]*SendTrack{},
			closeOnce:             sync.Once{},
			connClosedCh:          make(chan struct{}),
			logger:                log.New(os.Stdout, "TEST_MOQ_PEER: ", log.LstdFlags|log.Lshortfile),
		}
		go peer.controlLoop(mCtrlStream)
		go peer.close()
		env := &env{
			ctrl:      ctrl,
			peer:      peer,
			ctrStream: mCtrlStream,
			mc:        mc,
		}
		return env, func() {
			mc.EXPECT().CloseWithError(gomock.Any(), gomock.Any()).AnyTimes()
			_ = env.peer.CloseWithError(0, "")
		}
	}
	t.Run("close", func(t *testing.T) {
		env, teardown := setup(t)
		defer teardown()
		env.mc.EXPECT().CloseWithError(uint64(17), "TEST")
		_ = env.peer.CloseWithError(17, "TEST")
	})
	t.Run("read_unexpected_message", func(t *testing.T) {
		env, teardown := setup(t)
		defer teardown()
		mp := NewMockParser(env.ctrl)
		mpf := NewMockParserFactory(env.ctrl)
		env.peer.parserFactory = mpf
		env.mc.EXPECT().CloseWithError(uint64(ErrorCodeProtocolViolation), errUnexpectedMessage.Error())
		mpf.EXPECT().new(gomock.Any()).Return(mp)
		mp.EXPECT().parse().Return(&objectMessage{
			HasLength:       false,
			TrackID:         0,
			GroupSequence:   0,
			ObjectSequence:  0,
			ObjectSendOrder: 0,
			ObjectPayload:   []byte{},
		}, nil)
		go env.peer.ctrlStreamReadLoop(env.ctrStream)
		s, err := env.peer.ReadSubscription(context.Background())
		assert.Error(t, err)
		assert.Nil(t, s)
	})
	t.Run("subscribe", func(t *testing.T) {
		env, teardown := setup(t)
		defer teardown()
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			msg := <-env.peer.outgoingCtrlMessageCh
			ctrlMsg, ok := msg.(*ctrlMessage)
			assert.True(t, ok)
			assert.Equal(t, &subscribeRequestMessage{
				TrackNamespace: "",
				TrackName:      "",
				StartGroup:     Location{},
				StartObject:    Location{},
				EndGroup:       Location{},
				EndObject:      Location{},
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
		wg.Wait()
	})
	t.Run("subscribe_hint", func(t *testing.T) {
		env, teardown := setup(t)
		defer teardown()
		var wg sync.WaitGroup
		defer wg.Wait()
		wg.Add(1)
		go func() {
			defer wg.Done()
			msg := <-env.peer.outgoingCtrlMessageCh
			ctrlMsg, ok := msg.(*ctrlMessage)
			assert.True(t, ok)
			assert.Equal(t, &subscribeRequestMessage{
				TrackNamespace: "",
				TrackName:      "",
				StartGroup:     Location{},
				StartObject:    Location{},
				EndGroup:       Location{},
				EndObject:      Location{},
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
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			env.peer.incomingCtrlMessageCh <- &subscribeRequestMessage{
				TrackNamespace: "namespace",
				TrackName:      "track",
				StartGroup:     Location{},
				StartObject:    Location{},
				EndGroup:       Location{},
				EndObject:      Location{},
				Parameters:     map[uint64]parameter{},
			}
			res := <-env.peer.outgoingCtrlMessageCh
			assert.Equal(t, &subscribeOkMessage{
				TrackNamespace: "namespace",
				TrackName:      "track",
				TrackID:        17,
				Expires:        time.Second,
			}, res)
		}()

		s, err := env.peer.ReadSubscription(context.Background())
		assert.NoError(t, err)
		s.SetTrackID(17)
		s.SetExpires(time.Second)
		st := s.Accept()
		assert.NotNil(t, st)
		wg.Wait()
	})
	t.Run("handle_subscribe_hint", func(t *testing.T) {
		env, teardown := setup(t)
		defer teardown()
		ms := NewMockSendStream(env.ctrl)
		env.mc.EXPECT().OpenUniStream().Return(ms, nil)
		var wg sync.WaitGroup
		defer wg.Wait()
		wg.Add(1)
		go func() {
			defer wg.Done()
			env.peer.incomingCtrlMessageCh <- &subscribeRequestMessage{
				TrackNamespace: "ns",
				TrackName:      "track",
				StartGroup: Location{
					Mode:  LocationModeNone,
					Value: 0,
				},
				StartObject: Location{
					Mode:  LocationModeAbsolute,
					Value: 10,
				},
				EndGroup: Location{
					Mode:  LocationModeRelativePrevious,
					Value: 20,
				},
				EndObject: Location{
					Mode:  LocationModeRelativeNext,
					Value: 30,
				},
				Parameters: map[uint64]parameter{},
			}
			res := <-env.peer.outgoingCtrlMessageCh
			assert.Equal(t, &subscribeOkMessage{
				TrackNamespace: "ns",
				TrackName:      "track",
				TrackID:        17,
				Expires:        time.Second,
			}, res)
		}()

		s, err := env.peer.ReadSubscription(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, s.startGroup, Location{
			Mode:  LocationModeNone,
			Value: 0,
		})
		assert.Equal(t, s.startObject, Location{
			Mode:  LocationModeAbsolute,
			Value: 10,
		})
		assert.Equal(t, s.endGroup, Location{
			Mode:  LocationModeRelativePrevious,
			Value: 20,
		})
		assert.Equal(t, s.endObject, Location{
			Mode:  LocationModeRelativeNext,
			Value: 30,
		})
		s.SetTrackID(17)
		s.SetExpires(time.Second)
		st := s.Accept()
		assert.NotNil(t, st)
		wg.Wait()
	})
	t.Run("handle_subscribe_while_announcing", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Add(3)
		env, teardown := setup(t)
		defer teardown()
		ms := NewMockSendStream(env.ctrl)
		env.mc.EXPECT().OpenUniStream().Return(ms, nil)
		go func() {
			defer wg.Done()
			gotAnnounce := false
			gotSubscribeOk := false
			for msg := range env.peer.outgoingCtrlMessageCh {
				if ctrlMsg, ok := msg.(*ctrlMessage); ok && ctrlMsg.keyedMessage != nil {
					assert.Equal(t, &announceMessage{
						TrackNamespace:         "namespace",
						TrackRequestParameters: map[uint64]parameter{},
					}, ctrlMsg.keyedMessage)
					env.peer.incomingCtrlMessageCh <- &announceOkMessage{
						TrackNamespace: "namespace",
					}
					gotAnnounce = true
					fmt.Println("got announce message")
				} else {
					assert.Equal(t, &subscribeOkMessage{
						TrackNamespace: "other_namespace",
						TrackName:      "track",
						TrackID:        0,
						Expires:        0,
					}, msg)
					gotSubscribeOk = true
				}
				if gotAnnounce && gotSubscribeOk {
					break
				}
			}
		}()
		go func() {
			defer wg.Done()
			env.peer.incomingCtrlMessageCh <- &subscribeRequestMessage{
				TrackNamespace: "other_namespace",
				TrackName:      "track",
				StartGroup:     Location{},
				StartObject:    Location{},
				EndGroup:       Location{},
				EndObject:      Location{},
				Parameters:     map[uint64]parameter{},
			}
		}()
		go func() {
			defer wg.Done()
			s, err := env.peer.ReadSubscription(context.Background())
			assert.NoError(t, err)
			assert.NotNil(t, s)
			assert.NotNil(t, s.Accept())
		}()
		err := env.peer.Announce("namespace")
		assert.NoError(t, err)
		wg.Wait()
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
	t.Run("unsubscribe", func(t *testing.T) {
	})
	t.Run("goaway", func(t *testing.T) {
	})
	t.Run("handle_object_on_ctrl_stream", func(t *testing.T) {
		env, teardown := setup(t)
		defer teardown()
		env.mc.EXPECT().CloseWithError(uint64(ErrorCodeProtocolViolation), errUnexpectedMessage.Error()).Times(1)
		env.peer.incomingCtrlMessageCh <- &objectMessage{
			HasLength:       false,
			TrackID:         17,
			GroupSequence:   234,
			ObjectSequence:  123,
			ObjectSendOrder: 0,
			ObjectPayload:   []byte("payload"),
		}
		a, err := env.peer.ReadAnnouncement(context.Background())
		assert.Error(t, err)
		assert.ErrorContains(t, err, errUnexpectedMessage.Error())
		assert.Nil(t, a)
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
