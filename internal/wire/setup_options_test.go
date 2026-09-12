package wire

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateSetupOptions(t *testing.T) {
	cases := []struct {
		name    string
		options []KeyValuePair
		err     error
	}{
		{name: "empty"},
		{
			name: "all options",
			options: []KeyValuePair{
				{Type: SetupOptionTypePath, Bytes: []byte("/moq")},
				{Type: SetupOptionTypeAuthorizationToken, Bytes: []byte("a")},
				{Type: SetupOptionTypeAuthorizationToken, Bytes: []byte("b")},
				{Type: SetupOptionTypeMaxAuthTokenCacheSize, Varint: 1024},
				{Type: SetupOptionTypeAuthority, Bytes: []byte("example.com")},
				{Type: SetupOptionTypeMoqtImplementation, Bytes: []byte("moqtransport")},
			},
		},
		{
			name: "unknown options and their duplicates",
			options: []KeyValuePair{
				{Type: 0x1000, Varint: 1},
				{Type: 0x1000, Varint: 2},
				{Type: 0x1001, Bytes: []byte("x")},
			},
		},
		{
			name: "duplicate known option",
			options: []KeyValuePair{
				{Type: SetupOptionTypePath, Bytes: []byte("/a")},
				{Type: SetupOptionTypePath, Bytes: []byte("/b")},
			},
			err: ErrDuplicateOption,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateSetupOptions(tc.options)
			if tc.err == nil {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tc.err)
		})
	}
}

func TestSetupOptionAccessors(t *testing.T) {
	options := []KeyValuePair{
		{Type: SetupOptionTypePath, Bytes: []byte("/moq")},
		{Type: SetupOptionTypeMaxAuthTokenCacheSize, Varint: 1024},
	}
	b, ok := BytesSetupOption(options, SetupOptionTypePath)
	assert.True(t, ok)
	assert.Equal(t, []byte("/moq"), b)
	_, ok = BytesSetupOption(options, SetupOptionTypeAuthority)
	assert.False(t, ok)
	assert.Equal(t, uint64(1024), VarintSetupOption(options, SetupOptionTypeMaxAuthTokenCacheSize, 0))
	assert.Equal(t, uint64(0), VarintSetupOption(nil, SetupOptionTypeMaxAuthTokenCacheSize, 0))
}

func TestAllBytesSetupOptions(t *testing.T) {
	options := []KeyValuePair{
		{Type: SetupOptionTypeAuthorizationToken, Bytes: []byte("a")},
		{Type: SetupOptionTypePath, Bytes: []byte("/moq")},
		{Type: SetupOptionTypeAuthorizationToken, Bytes: []byte("b")},
	}
	assert.Equal(t, [][]byte{[]byte("a"), []byte("b")}, AllBytesSetupOptions(options, SetupOptionTypeAuthorizationToken))
	assert.Nil(t, AllBytesSetupOptions(options, SetupOptionTypeAuthority))
	assert.Nil(t, AllBytesSetupOptions(nil, SetupOptionTypeAuthorizationToken))
}
