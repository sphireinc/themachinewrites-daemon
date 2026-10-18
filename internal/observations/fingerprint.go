package observations

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"strings"
	"unicode"
)

func Fingerprint(rawURL, headline, sourceName string) (string, error) {
	canonical, err := CanonicalizeURL(rawURL)
	if err != nil {
		return "", err
	}
	normalHeadline := normalizeComparableText(headline)
	if normalHeadline == "" {
		return "", errors.New("headline is required for fingerprint")
	}
	sourceEvidence := normalizeSourceEvidence(canonical)
	payload := canonical + "\n" + normalHeadline + "\n" + sourceEvidence
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:]), nil
}

func normalizeSourceEvidence(canonicalURL string) string {
	parsed, err := url.Parse(canonicalURL)
	if err != nil {
		return canonicalURL
	}
	return strings.ToLower(parsed.Hostname())
}

func normalizeComparableText(value string) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			return ' '
		}
		return unicode.ToLower(r)
	}, value)
	return strings.Join(strings.Fields(value), " ")
}
