package operations

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/gitpub"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/store"
)

// Divergence is a read-only comparison between local publication state and
// the canonical posts in the remote public repository.
type Divergence struct {
	PendingUnpublished []string
	LocalOnlyPostIDs   []string
	RemoteOnlyPostIDs  []string
}

func (d Divergence) HasWarnings() bool {
	return len(d.PendingUnpublished) != 0 || len(d.LocalOnlyPostIDs) != 0 || len(d.RemoteOnlyPostIDs) != 0
}

// InspectDivergence reports discrepancies without repairing either source.
func InspectDivergence(ctx context.Context, runtime *store.Store, repository *gitpub.Repository) (Divergence, error) {
	if runtime == nil || repository == nil {
		return Divergence{}, errors.New("runtime store and public repository are required")
	}
	pending, err := runtime.PendingCandidates(ctx, 100)
	if err != nil {
		return Divergence{}, fmt.Errorf("read pending candidates: %w", err)
	}
	localIDs, err := runtime.OutstandingPublicationIDs(ctx)
	if err != nil {
		return Divergence{}, err
	}
	remoteIDs, err := gitpub.RemotePostIDs(ctx, repository)
	if err != nil {
		return Divergence{}, err
	}
	result := Divergence{
		PendingUnpublished: make([]string, 0, len(pending)),
		LocalOnlyPostIDs:   setDifference(localIDs, remoteIDs),
		RemoteOnlyPostIDs:  setDifference(remoteIDs, localIDs),
	}
	for _, candidate := range pending {
		result.PendingUnpublished = append(result.PendingUnpublished, candidate.Fingerprint)
	}
	sort.Strings(result.PendingUnpublished)
	return result, nil
}

func setDifference(left, right map[string]bool) []string {
	result := make([]string, 0)
	for value := range left {
		if !right[value] {
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

func (d Divergence) String() string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "pending_unpublished (%d): %s\n", len(d.PendingUnpublished), joinOrNone(d.PendingUnpublished))
	fmt.Fprintf(&builder, "local_only_post_ids (%d): %s\n", len(d.LocalOnlyPostIDs), joinOrNone(d.LocalOnlyPostIDs))
	fmt.Fprintf(&builder, "remote_only_post_ids (%d): %s\n", len(d.RemoteOnlyPostIDs), joinOrNone(d.RemoteOnlyPostIDs))
	return builder.String()
}

func joinOrNone(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	return strings.Join(values, ",")
}
