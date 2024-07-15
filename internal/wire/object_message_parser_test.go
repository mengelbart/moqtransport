package wire

import (
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestObjectStreamParser(t *testing.T) {
	cases := []struct {
		mr     *mockReader
		expect []*ObjectMessage
		err    error
	}{
		{
			mr: &mockReader{
				reads: [][]byte{},
				index: 0,
			},
			expect: []*ObjectMessage{},
			err:    io.EOF,
		},
		{
			mr: &mockReader{
				reads: [][]byte{
					(&ObjectMessage{
						Type:              0,
						SubscribeID:       0,
						TrackAlias:        0,
						GroupID:           0,
						ObjectID:          0,
						PublisherPriority: 0,
						ObjectPayload:     []byte{},
					}).Append([]byte{}),
				},
				index: 0,
			},
			expect: []*ObjectMessage{
				{
					Type:              0,
					SubscribeID:       0,
					TrackAlias:        0,
					GroupID:           0,
					ObjectID:          0,
					PublisherPriority: 0,
					ObjectPayload:     []byte{},
				},
			},
			err: io.EOF,
		},
		{
			mr: &mockReader{
				reads: [][]byte{
					(&StreamHeaderGroupMessage{
						SubscribeID:       0,
						TrackAlias:        0,
						GroupID:           0,
						PublisherPriority: 0,
					}).Append([]byte{}),
					(&StreamHeaderGroupObject{
						ObjectID:      0,
						ObjectPayload: []byte{0x00},
					}).Append([]byte{}),
					(&StreamHeaderGroupObject{
						ObjectID:      1,
						ObjectPayload: []byte{0x01},
					}).Append([]byte{}),
				},
				index: 0,
			},
			expect: []*ObjectMessage{
				{
					Type:              StreamHeaderGroupMessageType,
					SubscribeID:       0,
					TrackAlias:        0,
					GroupID:           0,
					ObjectID:          0,
					PublisherPriority: 0,
					ObjectPayload:     []byte{0x00},
				},
				{
					Type:              StreamHeaderGroupMessageType,
					SubscribeID:       0,
					TrackAlias:        0,
					GroupID:           0,
					ObjectID:          1,
					PublisherPriority: 0,
					ObjectPayload:     []byte{0x01},
				},
			},
			err: io.EOF,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			p := NewObjectStreamParser(tc.mr)
			res := []*ObjectMessage{}
			for {
				m, err := p.Parse()
				if err != nil {
					t.Logf("parser error: %v", err)
					assert.Equal(t, tc.err, err)
					assert.Equal(t, tc.expect, res)
					break
				}
				res = append(res, m)
			}
		})
	}
}
