package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseChatalog(t *testing.T) {
	cases := []struct {
		in        string
		expect    *chatalog[struct{}]
		expectErr error
	}{
		{
			in:        "",
			expect:    nil,
			expectErr: errInvalidVersion,
		},
		{
			in:        "version",
			expect:    nil,
			expectErr: errInvalidVersion,
		},
		{
			in: `version=1
alice
bob
charlie`,
			expect: &chatalog[struct{}]{
				version: 1,
				participants: map[string]struct{}{
					"alice":   {},
					"bob":     {},
					"charlie": {},
				},
			},
			expectErr: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			c, err := parseChatalog[struct{}](tc.in)
			if tc.expectErr != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tc.expectErr)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expect, c)
		})
	}
}

func TestSerializeChatalog(t *testing.T) {
	cases := []struct {
		in     *chatalog[struct{}]
		expect string
	}{
		{
			in:     &chatalog[struct{}]{version: 0, participants: nil},
			expect: "version=0",
		},
		{
			in: &chatalog[struct{}]{
				version: 1,
				participants: map[string]struct{}{
					"alice":   {},
					"bob":     {},
					"charlie": {},
				},
			},
			expect: `version=1
alice
bob
charlie`,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			c := tc.in.serialize()
			assert.Equal(t, tc.expect, c)
		})
	}
}

func TestParseDelta(t *testing.T) {
	cases := []struct {
		in        string
		expect    *delta
		expectErr error
	}{
		{
			in:        "",
			expect:    &delta{joined: []string{}, left: []string{}},
			expectErr: nil,
		},
		{
			in: `+daphne
-bob`,
			expect: &delta{
				joined: []string{"daphne"},
				left:   []string{"bob"},
			},
			expectErr: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			c, err := parseDelta(tc.in)
			if tc.expectErr != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tc.expectErr)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expect, c)
		})
	}
}
func TestSerializeDelta(t *testing.T) {
	cases := []struct {
		in     *delta
		expect string
	}{
		{
			in:     &delta{},
			expect: "",
		},
		{
			in: &delta{
				joined: []string{"daphne"},
				left:   []string{"bob"},
			},
			expect: `+daphne
-bob`,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			c := tc.in.serialize()
			assert.Equal(t, tc.expect, c)
		})
	}
}

func TestApplyDelta(t *testing.T) {
	cases := []struct {
		c         *chatalog[struct{}]
		d         *delta
		expect    *chatalog[struct{}]
		expectErr error
	}{
		{
			c:         &chatalog[struct{}]{},
			d:         &delta{},
			expect:    &chatalog[struct{}]{},
			expectErr: nil,
		},
		{
			c: &chatalog[struct{}]{
				version:      0,
				participants: map[string]struct{}{},
			},
			d: &delta{
				joined: []string{},
				left:   []string{},
			},
			expect: &chatalog[struct{}]{
				version:      0,
				participants: map[string]struct{}{},
			},
			expectErr: nil,
		},
		{
			c: &chatalog[struct{}]{
				version: 1,
				participants: map[string]struct{}{
					"alice":   {},
					"bob":     {},
					"charlie": {},
				},
			},
			d: &delta{
				joined: []string{"daphne"},
				left:   []string{"bob"},
			},
			expect: &chatalog[struct{}]{
				version: 1,
				participants: map[string]struct{}{
					"alice":   {},
					"charlie": {},
					"daphne":  {},
				},
			},
			expectErr: nil,
		},
		{
			c: &chatalog[struct{}]{
				version: 1,
				participants: map[string]struct{}{
					"alice":   {},
					"bob":     {},
					"charlie": {},
				},
			},
			d: &delta{
				joined: []string{"charlie"},
				left:   []string{"bob"},
			},
			expect: &chatalog[struct{}]{
				version: 1,
				participants: map[string]struct{}{
					"alice":   {},
					"charlie": {},
					"daphne":  {},
				},
			},
			expectErr: errDuplicateParticipantJoined,
		},
		{
			c: &chatalog[struct{}]{
				version: 1,
				participants: map[string]struct{}{
					"alice":   {},
					"bob":     {},
					"charlie": {},
				},
			},
			d: &delta{
				joined: []string{},
				left:   []string{"daphne"},
			},
			expect: &chatalog[struct{}]{
				version: 1,
				participants: map[string]struct{}{
					"alice":   {},
					"bob":     {},
					"charlie": {},
				},
			},
			expectErr: errUnknownParticipantLeft,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			err := tc.c.apply(tc.d)
			if tc.expectErr != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tc.expectErr)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expect, tc.c)
		})
	}
}
