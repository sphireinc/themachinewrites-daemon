package publication

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"
)

const ulidAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

func GenerateID(candidateKey string, publishedAt time.Time) (string, error) {
	if strings.TrimSpace(candidateKey) == "" {
		return "", errors.New("candidate key is required")
	}
	if publishedAt.Nanosecond()%int(time.Millisecond) != 0 {
		return "", errors.New("publication time must have millisecond precision")
	}
	millis := uint64(publishedAt.UTC().UnixMilli())
	if millis >= 1<<48 {
		return "", errors.New("publication time exceeds ULID range")
	}
	seed := sha256.Sum256([]byte(fmt.Sprintf("%s\n%d", candidateKey, millis)))
	id := make([]byte, 26)
	timeBytes := [6]byte{
		byte(millis >> 40), byte(millis >> 32), byte(millis >> 24),
		byte(millis >> 16), byte(millis >> 8), byte(millis),
	}
	var timestamp uint64
	for _, value := range timeBytes {
		timestamp = timestamp<<8 | uint64(value)
	}
	for index := 9; index >= 0; index-- {
		id[index] = ulidAlphabet[timestamp&31]
		timestamp >>= 5
	}
	randomBits := seed[:20]
	var accumulator uint32
	bits := 0
	output := 10
	for _, b := range randomBits {
		accumulator = accumulator<<8 | uint32(b)
		bits += 8
		for bits >= 5 && output < 26 {
			id[output] = ulidAlphabet[(accumulator>>(bits-5))&31]
			bits -= 5
			output++
		}
	}
	if output < 26 && bits > 0 {
		id[output] = ulidAlphabet[(accumulator<<(5-bits))&31]
	}
	return string(id), nil
}

func CanonicalPath(id string, publishedAt time.Time) (string, error) {
	if !validULID(id) {
		return "", errors.New("invalid ULID")
	}
	if ulidMilliseconds(id) != uint64(publishedAt.UTC().UnixMilli()) {
		return "", errors.New("ULID timestamp does not match publication time")
	}
	return fmt.Sprintf("posts/%04d/%02d/%02d/%s.md", publishedAt.UTC().Year(), publishedAt.UTC().Month(), publishedAt.UTC().Day(), id), nil
}

func validULID(id string) bool {
	if len(id) != 26 || id[0] < '0' || id[0] > '7' {
		return false
	}
	for _, r := range id {
		if !strings.ContainsRune(ulidAlphabet, r) {
			return false
		}
	}
	return true
}

func ulidMilliseconds(id string) uint64 {
	var value uint64
	for _, r := range id[:10] {
		value = value*32 + uint64(strings.IndexRune(ulidAlphabet, r))
	}
	return value
}
