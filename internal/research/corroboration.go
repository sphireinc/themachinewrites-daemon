package research

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"unicode"
)

type Evidence struct {
	SourceID string
	Text     string
}

type IndependenceGroup struct {
	Key       string
	SourceIDs []string
}

func GroupSyndicatedEvidence(items []Evidence) []IndependenceGroup {
	groups := map[string][]string{}
	for _, item := range items {
		key := syndicationFingerprint(item.Text)
		if key == "" {
			continue
		}
		groups[key] = append(groups[key], item.SourceID)
	}
	result := make([]IndependenceGroup, 0, len(groups))
	for key, ids := range groups {
		result = append(result, IndependenceGroup{Key: key, SourceIDs: uniqueStrings(ids)})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Key < result[j].Key })
	for index := range result {
		sort.Strings(result[index].SourceIDs)
	}
	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func IndependentEvidenceCount(items []Evidence) int {
	return len(GroupSyndicatedEvidence(items))
}

func syndicationFingerprint(text string) string {
	text = strings.ToLower(text)
	text = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
			return r
		}
		return ' '
	}, text)
	words := strings.Fields(text)
	if len(words) > 80 {
		words = words[:80]
	}
	if len(words) < 8 {
		return ""
	}
	digest := sha256.Sum256([]byte(strings.Join(words, " ")))
	return hex.EncodeToString(digest[:12])
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}
