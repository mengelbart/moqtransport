package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/exp/maps"
)

var (
	errInvalidVersion             = errors.New("invalid catalog version")
	errInvalidDeltaEntry          = errors.New("invalid entry in delta encoding")
	errDuplicateParticipantJoined = errors.New("a duplicate participant joined the chat")
	errDuplicateSubscriber        = errors.New("username is already subscribed to publisher")
	errUnknownParticipantLeft     = errors.New("an unknown participant left the chat")
)

type chatalog[V any] struct {
	version      int
	participants map[string]V
}

func (c *chatalog[V]) apply(d *delta) error {
	for _, p := range d.joined {
		if _, ok := c.participants[p]; ok {
			return errDuplicateParticipantJoined
		}
		c.participants[p] = *new(V)
	}
	for _, p := range d.left {
		if _, ok := c.participants[p]; !ok {
			return errUnknownParticipantLeft
		}
		delete(c.participants, p)
	}
	return nil
}

func (c *chatalog[V]) diff(other *chatalog[V]) *delta {
	result := &delta{
		joined: []string{},
		left:   []string{},
	}
	for p := range c.participants {
		if _, ok := other.participants[p]; !ok {
			result.left = append(result.left, p)
		}
	}
	for p := range other.participants {
		if _, ok := c.participants[p]; !ok {
			result.joined = append(result.joined, p)
		}
	}
	return result
}

func parseChatalog[V any](in string) (*chatalog[V], error) {
	scanner := bufio.NewScanner(bytes.NewReader([]byte(in)))
	if !scanner.Scan() {
		return nil, errInvalidVersion
	}
	versionKV := strings.Split(scanner.Text(), "=")
	if len(versionKV) != 2 {
		return nil, errInvalidVersion
	}
	version, err := strconv.Atoi(versionKV[1])
	if err != nil {
		return nil, err
	}
	participants := make(map[string]V)
	for scanner.Scan() {
		participants[scanner.Text()] = *new(V)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return &chatalog[V]{
		version:      version,
		participants: participants,
	}, nil
}

func (c *chatalog[V]) serialize() string {
	res := ""
	res += fmt.Sprintf("version=%v", c.version)
	if len(c.participants) == 0 {
		return res
	}
	participantList := maps.Keys(c.participants)
	slices.Sort(participantList)
	return res + "\n" + strings.Join(participantList, "\n")
}

type delta struct {
	joined []string
	left   []string
}

func parseDelta(in string) (*delta, error) {
	scanner := bufio.NewScanner(bytes.NewReader([]byte(in)))
	d := &delta{
		joined: []string{},
		left:   []string{},
	}
	for scanner.Scan() {
		line := scanner.Text()
		sign := line[0]
		if sign != '+' && sign != '-' {
			return nil, errInvalidDeltaEntry
		}
		if sign == '+' {
			d.joined = append(d.joined, line[1:])
		}
		if sign == '-' {
			d.left = append(d.left, line[1:])
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *delta) serialize() string {
	s := ""
	for i, p := range d.joined {
		if i > 0 && i < len(d.joined)-1 {
			s += "\n"
		}
		s += "+" + p
	}
	if len(d.left) > 0 {
		s += "\n"
	}
	for i, p := range d.left {
		if i > 0 && i < len(d.joined)-1 {
			s += "\n"
		}
		s += "-" + p
	}
	return s
}

func (d *delta) MarshalBinary() (data []byte, err error) {
	return []byte(d.serialize()), nil
}
