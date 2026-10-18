package sources

import (
	"encoding/xml"
	"fmt"
	"strings"
	"time"
)

type atomDocument struct {
	XMLName xml.Name    `xml:"feed"`
	Title   string      `xml:"title"`
	Entries []atomEntry `xml:"entry"`
}

type atomEntry struct {
	ID        string     `xml:"id"`
	Title     string     `xml:"title"`
	Updated   string     `xml:"updated"`
	Published string     `xml:"published"`
	Summary   string     `xml:"summary"`
	Content   string     `xml:"content"`
	Links     []atomLink `xml:"link"`
}

type atomLink struct {
	Rel  string `xml:"rel,attr"`
	Href string `xml:"href,attr"`
	Type string `xml:"type,attr"`
}

func ParseAtom(content []byte, source Source, retrievedAt time.Time) ([]Observation, error) {
	var document atomDocument
	if err := xml.Unmarshal(content, &document); err != nil {
		return nil, fmt.Errorf("parse Atom: %w", err)
	}
	entries := document.Entries
	if len(entries) > MaxObservationsPerFeed {
		entries = entries[:MaxObservationsPerFeed]
	}
	observations := make([]Observation, 0, len(entries))
	for _, entry := range entries {
		link := ""
		for _, candidate := range entry.Links {
			if candidate.Rel == "" || candidate.Rel == "alternate" {
				safe, err := safeFeedItemURL(candidate.Href)
				if err == nil {
					link = safe
					break
				}
			}
		}
		if link == "" {
			continue
		}
		headline := boundedPlainText(entry.Title, 240)
		if headline == "" {
			continue
		}
		published := entry.Published
		if strings.TrimSpace(published) == "" {
			published = entry.Updated
		}
		publishedAt, err := parseFeedTime(published)
		if err != nil {
			publishedAt = retrievedAt.UTC()
		}
		summary := entry.Summary
		if strings.TrimSpace(summary) == "" {
			summary = entry.Content
		}
		observations = append(observations, Observation{
			GUID:        strings.TrimSpace(entry.ID),
			Headline:    headline,
			URL:         link,
			SourceID:    source.ID,
			SourceName:  source.Name,
			Category:    source.Category,
			PublishedAt: publishedAt,
			RetrievedAt: retrievedAt.UTC(),
			Excerpt:     boundedPlainText(summary, 500),
		})
	}
	return observations, nil
}
