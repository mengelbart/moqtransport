package moqtransport

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os"
	"testing"

	"github.com/quic-go/quic-go/quicvarint"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestServerPeer(t *testing.T) {
	t.Run("nil_connection", func(t *testing.T) {
		_, err := newServerPeer(nil, nil)
		assert.Error(t, err)
	})
	t.Run("AcceptStream_error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mc := NewMockConnection(ctrl)
		mc.EXPECT().AcceptStream(gomock.Any()).Times(1).Return(nil, errors.New("error"))
		_, err := newServerPeer(mc, nil)
		assert.Error(t, err)
	})
	t.Run("exchange_setup_messages", func(t *testing.T) {
		acceptCalled := make(chan struct{})
		parseCalled := make(chan struct{})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ctrl := gomock.NewController(t)
		mc := NewMockConnection(ctrl)
		ms1 := NewMockStream(ctrl)
		ms2 := NewMockStream(ctrl)
		mp := NewMockParser(ctrl)
		mc.EXPECT().AcceptStream(gomock.Any()).Times(1).Return(ms1, nil)
		mc.EXPECT().AcceptUniStream(gomock.Any()).Times(1).Do(func(context.Context) (receiveStream, error) {
			close(acceptCalled)
			<-ctx.Done()
			return ms2, nil
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
		mp.EXPECT().parse().Times(1).DoAndReturn(func() (message, error) {
			close(parseCalled)
			<-ctx.Done()
			return nil, errors.New("connection closed")
		})
		ms1.EXPECT().Write(gomock.Any()).DoAndReturn(func(buf []byte) (int, error) {
			msg := parse(t, buf)
			assert.Equal(t, &serverSetupMessage{
				SelectedVersion: DRAFT_IETF_MOQ_TRANSPORT_01,
				SetupParameters: map[uint64]parameter{},
			}, msg)
			return len(buf), nil
		})
		ms1.EXPECT().Close()

		p, err := newServerPeer(mc, getParserFactory(mp))
		assert.NoError(t, err)
		assert.NotNil(t, p)
		p.Run(false)
		<-parseCalled
		<-acceptCalled
		err = p.Close()
		assert.NoError(t, err)
	})
}

func TestClientPeer(t *testing.T) {
	t.Run("nil_connection", func(t *testing.T) {
		_, err := newClientPeer(nil, nil, 0)
		assert.Error(t, err)
	})
	t.Run("open_control_stream", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mc := NewMockConnection(ctrl)
		mc.EXPECT().OpenStreamSync(gomock.Any()).Times(1).Return(nil, errors.New("error"))
		_, err := newClientPeer(mc, nil, 0)
		assert.Error(t, err)
	})
	t.Run("exchange_setup_messages", func(t *testing.T) {
		acceptCalled := make(chan struct{})
		parseCalled := make(chan struct{})
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ctrl := gomock.NewController(t)
		mc := NewMockConnection(ctrl)
		ms1 := NewMockStream(ctrl)
		ms2 := NewMockStream(ctrl)
		mp := NewMockParser(ctrl)
		mc.EXPECT().OpenStreamSync(gomock.Any()).Times(1).Return(ms1, nil)
		mc.EXPECT().AcceptUniStream(gomock.Any()).Times(1).Do(func(context.Context) (receiveStream, error) {
			close(acceptCalled)
			<-ctx.Done()
			return ms2, nil
		})
		ms1.EXPECT().Write(gomock.Any()).DoAndReturn(func(buf []byte) (int, error) {
			msg := parse(t, buf)
			assert.Equal(t, &clientSetupMessage{
				SupportedVersions: []version{DRAFT_IETF_MOQ_TRANSPORT_01},
				SetupParameters: map[uint64]parameter{
					roleParameterKey: varintParameter{
						k: roleParameterKey,
						v: 0,
					},
				},
			}, msg)
			return len(buf), nil
		})
		mp.EXPECT().parse().Times(1).Return(&serverSetupMessage{
			SelectedVersion: DRAFT_IETF_MOQ_TRANSPORT_01,
			SetupParameters: map[uint64]parameter{},
		}, nil)
		mp.EXPECT().parse().Times(1).DoAndReturn(func() (message, error) {
			close(parseCalled)
			return nil, errors.New("connection closed")
		})
		ms1.EXPECT().Close()

		p, err := newClientPeer(mc, getParserFactory(mp), 0)
		assert.NoError(t, err)
		assert.NotNil(t, p)
		p.Run(false)
		<-acceptCalled
		<-parseCalled
		err = p.Close()
		assert.NoError(t, err)
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

func getParserFactory(p parser) func(messageReader, *log.Logger) parser {
	return func(mr messageReader, l *log.Logger) parser {
		return p
	}
}
