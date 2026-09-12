package wire

import (
	"errors"
	"fmt"
)

const (
	SetupOptionTypePath                  uint64 = 0x01
	SetupOptionTypeAuthorizationToken    uint64 = 0x03
	SetupOptionTypeMaxAuthTokenCacheSize uint64 = 0x04
	SetupOptionTypeAuthority             uint64 = 0x05
	SetupOptionTypeMoqtImplementation    uint64 = 0x07
)

var setupOptionNames = map[uint64]string{
	SetupOptionTypePath:                  "PATH",
	SetupOptionTypeAuthorizationToken:    "AUTHORIZATION_TOKEN",
	SetupOptionTypeMaxAuthTokenCacheSize: "MAX_AUTH_TOKEN_CACHE_SIZE",
	SetupOptionTypeAuthority:             "AUTHORITY",
	SetupOptionTypeMoqtImplementation:    "MOQT_IMPLEMENTATION",
}

var repeatableSetupOptions = map[uint64]bool{
	SetupOptionTypeAuthorizationToken: true,
}

var (
	ErrDuplicateOption = errors.New("duplicate option")
)

func ValidateSetupOptions(options []KeyValuePair) error {
	seen := make(map[uint64]struct{}, len(options))
	for _, o := range options {
		name, known := setupOptionNames[o.Type]
		if !known || repeatableSetupOptions[o.Type] {
			continue
		}
		if _, dup := seen[o.Type]; dup {
			return fmt.Errorf("%w: setup option %s", ErrDuplicateOption, name)
		}
		seen[o.Type] = struct{}{}
	}
	return nil
}

func findSetupOption(options []KeyValuePair, t uint64) (*KeyValuePair, bool) {
	for i := range options {
		if options[i].Type == t {
			return &options[i], true
		}
	}
	return nil, false
}

// BytesSetupOption returns the first option of type t and whether it was present.
func BytesSetupOption(options []KeyValuePair, t uint64) ([]byte, bool) {
	if o, ok := findSetupOption(options, t); ok {
		return o.Bytes, true
	}
	return nil, false
}

// AllBytesSetupOptions returns the values of every option of type t in order.
func AllBytesSetupOptions(options []KeyValuePair, t uint64) [][]byte {
	var values [][]byte
	for i := range options {
		if options[i].Type == t {
			values = append(values, options[i].Bytes)
		}
	}
	return values
}

// VarintSetupOption returns the first option of type t, or def when absent.
func VarintSetupOption(options []KeyValuePair, t uint64, def uint64) uint64 {
	if o, ok := findSetupOption(options, t); ok {
		return o.Varint
	}
	return def
}
