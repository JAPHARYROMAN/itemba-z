// Package identity provides collision-resistant internal identifiers.
package identity

import (
	"crypto/rand"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

func IsUUID(value string) bool { return uuidPattern.MatchString(value) }

var ErrInvalidUUID = errors.New("value is not a canonicalizable UUID")

func CanonicalUUID(value string) (string, error) {
	value = strings.TrimSpace(value)
	if !IsUUID(value) {
		return "", ErrInvalidUUID
	}
	return strings.ToLower(value), nil
}

// NormalizeClaim canonicalizes valid UUID spellings and also gives in-memory
// domain tests deterministic lower-case keys for their opaque fixture IDs.
func NormalizeClaim(value string) string { return strings.ToLower(strings.TrimSpace(value)) }

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
