package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/engine"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/gitpub"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/memory"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/observations"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/prompts"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/publication"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/sources"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/store"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/supervisor"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/telemetry"
)

type Runtime struct {
	Sources         sources.Configuration
	Fetcher         sources.FeedFetcher
	Store           *store.Store
	Engine          engine.Engine
	Publisher       gitpub.Publisher
	Outbox          gitpub.Outbox
	PublicRoot      string
	ModelName       string
	Now             func() time.Time
	DryRun          bool
	Emitter         *telemetry.Emitter
	Metrics         *telemetry.Metrics
	MaxCandidates   int
	Budget          *supervisor.Budget
	MaxOutputTokens int
	MemoryMaxItems  int
}

func (r Runtime) Poll(ctx context.Context) []error {
	if r.Fetcher == nil || r.Store == nil {
		return []error{errors.New("daemon poll dependencies are incomplete")}
	}
	now := r.now()
	var errorsFound []error
	for _, source := range r.Sources.Sources {
		if !source.Enabled {
			continue
		}
		started := time.Now()
		result, err := sources.Poll(ctx, source, r.Fetcher, cursorAdapter{runtime: r.Store}, now)
		if r.Emitter != nil {
			r.Emitter.Emit(ctx, telemetry.EventSourcePoll, "", time.Since(started), "source_id", source.ID)
		}
		if err != nil {
			if r.Metrics != nil {
				r.Metrics.SourceErrored()
			}
			errorsFound = append(errorsFound, fmt.Errorf("source %s: %w", source.ID, err))
			continue
		}
		for _, item := range result.Observations {
			inserted, err := observations.Ingest(ctx, r.Store, observations.Candidate{
				URL: item.URL, Headline: item.Headline, SourceID: item.SourceID, SourceName: item.SourceName,
				Category: item.Category, PublishedAt: item.PublishedAt, Excerpt: item.Excerpt, RetrievedAt: item.RetrievedAt,
			})
			if err != nil {
				errorsFound = append(errorsFound, err)
				continue
			}
			if inserted && r.Emitter != nil {
				r.Emitter.Emit(ctx, telemetry.EventCandidate, item.GUID, 0, "source_id", source.ID)
			}
		}
	}
	return errorsFound
}

type cursorAdapter struct{ runtime *store.Store }

func (a cursorAdapter) SourceCursor(ctx context.Context, sourceID string) (sources.CursorState, bool, error) {
	cursor, found, err := a.runtime.SourceCursor(ctx, sourceID)
	return sources.CursorState{ETag: cursor.ETag, LastModified: cursor.LastModified}, found, err
}

func (a cursorAdapter) AdvanceSourceCursor(ctx context.Context, sourceID, etag, lastModified string, at time.Time) error {
	return a.runtime.AdvanceSourceCursor(ctx, sourceID, etag, lastModified, at)
}

func (a cursorAdapter) RecordSourceFailure(ctx context.Context, sourceID string, at time.Time, message string) error {
	return a.runtime.RecordSourceFailure(ctx, sourceID, at, message)
}

func (r Runtime) DecideAndPublish(ctx context.Context) error {
	if r.Store == nil || r.Engine == nil {
		return errors.New("daemon decision dependencies are incomplete")
	}
	candidates, err := r.Store.PendingCandidates(ctx, r.maxCandidates())
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		return nil
	}
	fingerprints := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		fingerprints = append(fingerprints, candidate.Fingerprint)
	}
	if err := r.Store.MarkCandidatesEvaluating(ctx, fingerprints, r.now()); err != nil {
		return err
	}
	items, err := toObservationCandidates(candidates)
	if err != nil {
		return err
	}
	record, bundle, err := observations.BuildBundle(items, observations.BundleLimits{MaxItems: len(items), MaxBytes: 128 * 1024})
	if err != nil {
		return err
	}
	topics := make([]string, 0)
	for _, item := range items {
		if item.Category != "" {
			topics = append(topics, item.Category)
		}
	}
	retrieved, err := memory.Retrieve(ctx, r.Store, memory.RetrievalOptions{Topics: topics, MaxItems: r.memoryItems(), MaxBytes: 16 * 1024})
	if err != nil {
		return err
	}
	record.Memory = retrieved.JSON
	corpus, err := publication.LoadCorpus(r.PublicRoot)
	if err != nil {
		return err
	}
	if revisit := publication.BuildRevisitCandidates(corpus.Posts, r.now(), 30*24*time.Hour); len(revisit) > 0 {
		record.RevisitPrompt = publication.RevisitPrompt(revisit)
	}
	bundle, err = json.Marshal(record)
	if err != nil {
		return err
	}
	promptBundle, err := prompts.Load()
	if err != nil {
		return err
	}
	started := time.Now()
	if r.Emitter != nil {
		r.Emitter.Emit(ctx, telemetry.EventModelStarted, "", 0)
	}
	if r.Metrics != nil {
		r.Metrics.ModelInvoked()
	}
	if r.Budget != nil {
		inputTokens := len(bundle)/4 + 1
		outputTokens := r.MaxOutputTokens
		if outputTokens < 1 {
			outputTokens = 4000
		}
		if err := r.Budget.Reserve(r.now(), inputTokens, outputTokens); err != nil {
			return err
		}
	}
	decision, err := engine.Decide(ctx, r.Engine, engine.Input{Instructions: promptBundle.Text, Bundle: string(bundle), PromptVersion: promptBundle.Version}, r.PublicRoot)
	if r.Metrics != nil {
		r.Metrics.ObserveDurationMilliseconds(uint64(time.Since(started).Milliseconds()))
	}
	if err != nil {
		_ = r.Store.SetCandidateDecision(ctx, fingerprints, "pending", r.now())
		return err
	}
	if decision.ContentCandidate == nil {
		if err := r.Store.SetCandidateDecision(ctx, fingerprints, "deferred", r.now()); err != nil {
			return err
		}
		if r.Emitter != nil {
			r.Emitter.Emit(ctx, telemetry.EventModelDeferred, "", time.Since(started), "reason", decision.Proposal.Reason)
		}
		return nil
	}
	if r.DryRun {
		_ = r.Store.SetCandidateDecision(ctx, fingerprints, "pending", r.now())
		return nil
	}
	if r.Publisher == nil || r.Outbox == nil || strings.TrimSpace(r.PublicRoot) == "" {
		return errors.New("daemon publication dependencies are incomplete")
	}
	at := r.now().UTC().Truncate(time.Millisecond)
	postID, err := publication.GenerateID(candidates[0].Fingerprint, at)
	if err != nil {
		return err
	}
	rendered, err := publication.Render(*decision.ContentCandidate, publication.RenderInput{
		ID: postID, PublishedAt: at, ModelIdentifier: r.ModelName, PromptVersion: promptBundle.Version,
	})
	if err != nil {
		return err
	}
	if err := publication.WriteRenderedPost(r.PublicRoot, rendered); err != nil {
		return err
	}
	pending, err := publication.LoadCorpus(r.PublicRoot)
	if err != nil {
		return err
	}
	feedPaths, err := publication.MaterializeFeeds(r.PublicRoot, pending.Posts, at)
	if err != nil {
		return err
	}
	paths := []string{rendered.Path}
	for path := range feedPaths {
		paths = append(paths, path)
	}
	if r.Emitter != nil {
		r.Emitter.Emit(ctx, telemetry.EventPostStaged, postID, 0)
	}
	result, err := gitpub.Publish(ctx, r.Publisher, r.Outbox, gitpub.PublishRequest{PostID: postID, Paths: paths, MaxPushAttempts: 3})
	if err != nil {
		_ = r.Store.SetCandidateDecision(ctx, fingerprints, "failed", r.now())
		return err
	}
	if err := r.Store.SetCandidateDecision(ctx, fingerprints, "published", r.now()); err != nil {
		return err
	}
	if r.Metrics != nil {
		r.Metrics.Published()
	}
	if r.Emitter != nil {
		r.Emitter.Emit(ctx, telemetry.EventPushConfirmed, postID, 0, "commit_sha", result.CommitSHA)
	}
	return nil
}

func (r Runtime) memoryItems() int {
	if r.MemoryMaxItems < 1 || r.MemoryMaxItems > 20 {
		return 8
	}
	return r.MemoryMaxItems
}

func (r Runtime) maxCandidates() int {
	if r.MaxCandidates < 1 || r.MaxCandidates > 50 {
		return 20
	}
	return r.MaxCandidates
}

func (r Runtime) now() time.Time {
	if r.Now == nil {
		return time.Now().UTC()
	}
	return r.Now().UTC()
}

func toObservationCandidates(items []store.Candidate) ([]observations.Candidate, error) {
	result := make([]observations.Candidate, 0, len(items))
	for _, item := range items {
		published, err := time.Parse(time.RFC3339Nano, item.PublishedAt)
		if err != nil {
			return nil, err
		}
		retrieved, err := time.Parse(time.RFC3339Nano, item.DiscoveredAt)
		if err != nil {
			return nil, err
		}
		result = append(result, observations.Candidate{
			URL: item.URL, Headline: item.Headline, SourceID: item.SourceID, SourceName: item.SourceName,
			Category: item.Category, PublishedAt: published, Excerpt: item.Excerpt, RetrievedAt: retrieved,
		})
	}
	return result, nil
}

func LogCycleFailure(logger *slog.Logger, err error) {
	if logger != nil && err != nil {
		logger.Error("cycle failed", "event", string(telemetry.EventCycleFailed), "error", err)
	}
}
