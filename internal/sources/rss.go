package sources

import (
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"
)

const MaxObservationsPerFeed = 100

var (
	blockPattern   = regexp.MustCompile(`(?is)<(script|style|iframe|object|embed)\b[^>]*>.*?</(?:script|style|iframe|object|embed)\s*>`)
	htmlTagPattern = regexp.MustCompile(`(?s)<[^>]*>`)
	spacePattern   = regexp.MustCompile(`\s+`)
	punctPattern   = regexp.MustCompile(`\s+([,.;:!?])`)
)

type rssDocument struct {
	Channel struct {
		Title string    `xml:"title"`
		Items []rssItem `xml:"item"`
	} `xml:"channel"`
}

type rssItem struct {
	GUID        string `xml:"guid"`
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

func ParseRSS(content []byte, source Source, retrievedAt time.Time) ([]Observation, error) {
	var document rssDocument
	decoder := xml.NewDecoder(strings.NewReader(string(content)))
	decoder.Strict = true
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("parse RSS: %w", err)
	}
	items := document.Channel.Items
	if len(items) > MaxObservationsPerFeed {
		items = items[:MaxObservationsPerFeed]
	}
	observations := make([]Observation, 0, len(items))
	for _, item := range items {
		itemURL, err := safeFeedItemURL(item.Link)
		if err != nil {
			continue
		}
		headline := boundedPlainText(item.Title, 240)
		if headline == "" {
			continue
		}
		guid := strings.TrimSpace(item.GUID)
		if guid == "" {
			guid = itemURL
		}
		published, err := parseFeedTime(item.PubDate)
		if err != nil {
			published = retrievedAt.UTC()
		}
		observations = append(observations, Observation{
			GUID:        guid,
			Headline:    headline,
			URL:         itemURL,
			SourceID:    source.ID,
			SourceName:  source.Name,
			Category:    source.Category,
			PublishedAt: published,
			RetrievedAt: retrievedAt.UTC(),
			Excerpt:     boundedPlainText(item.Description, 500),
		})
	}
	return observations, nil
}

func safeFeedItemURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" {
		return "", errors.New("unsafe item URL")
	}
	parsed.User = nil
	parsed.Fragment = ""
	query := parsed.Query()
	for key := range query {
		if strings.HasPrefix(strings.ToLower(key), "utm_") {
			query.Del(key)
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func boundedPlainText(value string, limit int) string {
	text := blockPattern.ReplaceAllString(value, "")
	text = htmlTagPattern.ReplaceAllString(text, " ")
	text = html.UnescapeString(text)
	text = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, text)
	text = strings.TrimSpace(spacePattern.ReplaceAllString(text, " "))
	text = punctPattern.ReplaceAllString(text, "$1")
	runes := []rune(text)
	if len(runes) > limit {
		runes = runes[:limit]
	}
	return string(runes)
}

func parseFeedTime(value string) (time.Time, error) {
	for _, layout := range []string{time.RFC1123Z, time.RFC1123, time.RFC3339, time.RFC822Z} {
		if parsed, err := time.Parse(layout, strings.TrimSpace(value)); err == nil {
			return parsed.UTC(), nil
		}
	}
	return time.Time{}, io.ErrUnexpectedEOF
}
