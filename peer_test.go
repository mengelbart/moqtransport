package moqtransport

import (
	"context"
	"errors"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestNewServerPeer(t *testing.T) {
	t.Run("nil connection", func(t *testing.T) {
		_, err := newServerPeer(context.Background(), nil, nil)
		assert.Error(t, err)
	})
	t.Run("AcceptStream error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mc := NewMockConnection(ctrl)
		mc.EXPECT().AcceptStream(gomock.Any()).Times(1).Return(nil, errors.New("error"))
		_, err := newServerPeer(context.Background(), mc, nil)
		assert.Error(t, err)
	})
	t.Run("", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mc := NewMockConnection(ctrl)
		ms := NewMockStream(ctrl)
		mp := NewMockParser(ctrl)
		mc.EXPECT().AcceptStream(gomock.Any()).Times(1).Return(ms, nil)
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
		p, err := newServerPeer(context.Background(), mc, func(mr messageReader, l *log.Logger) parser {
			return mp
		})
		// ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		// defer cancel()
		// p.Run(ctx, false)
		assert.NoError(t, err)
		assert.NotNil(t, p)
	})
}
