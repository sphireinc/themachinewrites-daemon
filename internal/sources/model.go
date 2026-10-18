package sources

import "time"

type Observation struct {
	GUID        string
	Headline    string
	URL         string
	SourceID    string
	SourceName  string
	Category    string
	PublishedAt time.Time
	Excerpt     string
	RetrievedAt time.Time
}
