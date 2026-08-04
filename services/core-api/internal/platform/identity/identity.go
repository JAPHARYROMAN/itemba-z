// Package identity provides collision-resistant internal identifiers.
package identity

import (
	"crypto/rand"
	"fmt"
)

type Generator interface {
	New() (string, error)
}

type UUIDGenerator struct{}

func (UUIDGenerator) New() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		value[0:4], value[4:6], value[6:8], value[8:10], value[10:16]), nil
}

type SequenceGenerator struct {
	Values []string
	next   int
}

func (g *SequenceGenerator) New() (string, error) {
	if g.next >= len(g.Values) {
		return "", fmt.Errorf("deterministic id sequence exhausted")
	}
	value := g.Values[g.next]
	g.next++
	return value, nil
}
