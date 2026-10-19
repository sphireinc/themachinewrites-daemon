package publication

import "time"

type RelatedIndex struct {
	ByID   map[string][]string
	Counts map[string]int
}

func BuildRelatedIndex(posts []PostRecord, generatedAt time.Time) RelatedIndex {
	relations := BuildRelations(posts, generatedAt)
	index := RelatedIndex{ByID: relations.Relations, Counts: make(map[string]int)}
	for id, ids := range relations.Relations {
		index.Counts[id] = len(ids)
	}
	return index
}
