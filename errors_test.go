package moqtransport

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRequestErrorIs(t *testing.T) {
	err := &RequestError{Code: RequestErrorCodeDoesNotExist, Reason: "no such track"}
	assert.ErrorIs(t, err, &RequestError{Code: RequestErrorCodeDoesNotExist})
	assert.NotErrorIs(t, err, &RequestError{Code: RequestErrorCodeUnauthorized})
	assert.False(t, errors.Is(err, ErrRequestClosed))
	assert.Equal(t, "request error 0x10: no such track", err.Error())
}

func TestRequestErrorRetryAfter(t *testing.T) {
	cases := []struct {
		interval uint64
		want     time.Duration
		ok       bool
	}{
		{0, 0, false},
		{1, 0, true},
		{1001, time.Second, true},
	}
	for _, c := range cases {
		d, ok := (&RequestError{RetryInterval: c.interval}).RetryAfter()
		assert.Equal(t, c.ok, ok)
		assert.Equal(t, c.want, d)
	}
}
