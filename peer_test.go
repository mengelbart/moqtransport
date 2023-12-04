package moqtransport

import (
	"context"
	"errors"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestNewServerPeer(t *testing.T) {
	t.Run("nil connection", func(t *testing.T) {
		_, err := newServerPeer(nil, nil)
		assert.Error(t, err)
	})
	t.Run("AcceptStream error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mc := NewMockConnection(ctrl)
		mc.EXPECT().AcceptStream(gomock.Any()).Times(1).Return(nil, errors.New("error"))
		_, err := newServerPeer(mc, nil)
		assert.Error(t, err)
	})
	t.Run("Close gracefully", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ctrl := gomock.NewController(t)
		mc := NewMockConnection(ctrl)
		ms := NewMockStream(ctrl)
		mp := NewMockParser(ctrl)
		mc.EXPECT().AcceptStream(gomock.Any()).Times(1).Return(ms, nil)
		mc.EXPECT().AcceptUniStream(gomock.Any()).Times(1).Do(func(context.Context) (receiveStream, error) {
			<-ctx.Done()
			return nil, errClosed
		})
		ms.EXPECT().Read(gomock.Any()).AnyTimes()
		ms.EXPECT().Write(gomock.Any()).AnyTimes()
		mp.EXPECT().parse().Times(1).Return(&clientSetupMessage{
			SupportedVersions: []version{DRAFT_IETF_MOQ_TRANSPORT_01},
			SetupParameters: map[uint64]parameter{
				roleParameterKey: varintParameter{
					k: roleParameterKey,
					v: uint64(clientRole),
				},
			},
		}, nil)
		mp.EXPECT().parse().Times(1).DoAndReturn(func() (message, error) {
			<-ctx.Done()
			return nil, errClosed
		})
		ms.EXPECT().Close()
		p, err := newServerPeer(mc, func(mr messageReader, l *log.Logger) parser {
			return mp
		})
		assert.NoError(t, err)
		assert.NotNil(t, p)

		p.Run(false)
		time.Sleep(time.Second)
		err = p.Close()
		assert.NoError(t, err)
	})
}
