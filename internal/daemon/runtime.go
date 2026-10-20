package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/engine"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/gitpub"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/memory"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/observations"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/prompts"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/publication"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/research"
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
	HealthPath      string
	WorkerID        string
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
	if err := r.writeHealth(func(health *telemetry.Health) {
		health.LastPoll = timePointer(now)
		if len(errorsFound) > 0 {
			health.LastErrorKind = "source_failure"
		} else {
			health.LastErrorKind = ""
		}
	}); err != nil {
		errorsFound = append(errorsFound, err)
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
	at := r.now().UTC().Truncate(time.Millisecond)
	reservations := make([]store.CandidateReservation, 0, len(candidates))
	for _, candidate := range candidates {
		postID := candidate.DecisionPostID
		publicationAt := at
		if candidate.PublicationAt != "" {
			publicationAt, err = time.Parse(time.RFC3339Nano, candidate.PublicationAt)
			if err != nil {
				return err
			}
		}
		if postID == "" {
			postID, err = publication.GenerateID(candidate.Fingerprint, publicationAt)
			if err != nil {
				return err
			}
		}
		reservations = append(reservations, store.CandidateReservation{Fingerprint: candidate.Fingerprint, PostID: postID, At: publicationAt})
	}
	fingerprints := make([]string, 0, len(reservations))
	for _, reservation := range reservations {
		fingerprints = append(fingerprints, reservation.Fingerprint)
	}
	if err := r.Store.ReserveCandidates(ctx, reservations); err != nil {
		return err
	}
	items, err := toObservationCandidates(candidates)
	if err != nil {
		_ = r.Store.SetCandidateDecision(ctx, fingerprints, "failed", r.now())
		return err
	}
	record, bundle, err := observations.BuildBundle(items, observations.BundleLimits{MaxItems: len(items), MaxBytes: 128 * 1024})
	if err != nil {
		_ = r.Store.SetCandidateDecision(ctx, fingerprints, "failed", r.now())
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
		_ = r.Store.SetCandidateDecision(ctx, fingerprints, "failed", r.now())
		return err
	}
	record.Memory = retrieved.JSON
	corpus, err := publication.LoadCorpus(r.PublicRoot)
	if err != nil {
		_ = r.Store.SetCandidateDecision(ctx, fingerprints, "failed", r.now())
		return err
	}
	revisitQueue := buildRevisitQueue(corpus.Posts, items, r.now())
	if len(revisitQueue) > 0 {
		payload, _ := json.Marshal(revisitQueue)
		record.RevisitPrompt = "Review these scheduled revisit opportunities and choose investigate, defer, publish a follow-up analysis/reflection, or publish a correction linked to the original: " + string(payload)
	}
	bundle, err = json.Marshal(record)
	if err != nil {
		_ = r.Store.SetCandidateDecision(ctx, fingerprints, "failed", r.now())
		return err
	}
	promptBundle, err := prompts.Load()
	if err != nil {
		_ = r.Store.SetCandidateDecision(ctx, fingerprints, "failed", r.now())
		return err
	}
	if r.Publisher != nil && r.Outbox != nil && strings.TrimSpace(r.PublicRoot) != "" {
		workerID := r.WorkerID
		if strings.TrimSpace(workerID) == "" {
			workerID = "daemon"
		}
		lease, err := supervisor.AcquireLease(filepath.Join(r.PublicRoot, ".git"), workerID)
		if err != nil {
			_ = r.Store.SetCandidateDecision(ctx, fingerprints, "pending", r.now())
			return err
		}
		defer lease.Release()
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
			_ = r.Store.SetCandidateDecision(ctx, fingerprints, "pending", r.now())
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
	if err := r.writeHealth(func(health *telemetry.Health) {
		health.LastModelCall = timePointer(r.now())
		health.LastErrorKind = ""
	}); err != nil {
		_ = r.Store.SetCandidateDecision(ctx, fingerprints, "pending", r.now())
		return err
	}
	if decision.ContentCandidate == nil {
		if err := r.Store.SetCandidateDecision(ctx, fingerprints, "deferred", r.now()); err != nil {
			return err
		}
		if err := r.writeHealth(func(health *telemetry.Health) {
			health.PausedReason = "insufficient_evidence"
		}); err != nil {
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
		_ = r.Store.SetCandidateDecision(ctx, fingerprints, "failed", r.now())
		return errors.New("daemon publication dependencies are incomplete")
	}
	postID := reservations[0].PostID
	at = reservations[0].At
	rendered, err := publication.Render(*decision.ContentCandidate, publication.RenderInput{
		ID: postID, PublishedAt: at, CorrectionOf: decision.ContentCandidate.CorrectionOf,
		ModelIdentifier: r.ModelName, PromptVersion: promptBundle.Version,
		IndependentSources: independentSourceCount(items),
	})
	if err != nil {
		_ = r.Store.SetCandidateDecision(ctx, fingerprints, "failed", r.now())
		return err
	}
	if decision.ContentCandidate.CorrectionOf != "" {
		if err := publication.ValidateCorrectionDraft(corpus.Posts, postID, at, decision.ContentCandidate.CorrectionOf); err != nil {
			_ = r.Store.SetCandidateDecision(ctx, fingerprints, "failed", r.now())
			return err
		}
	}
	if err := publication.WriteRenderedPost(r.PublicRoot, rendered); err != nil {
		_ = r.Store.SetCandidateDecision(ctx, fingerprints, "failed", r.now())
		return err
	}
	pending, err := publication.LoadCorpus(r.PublicRoot)
	if err != nil {
		_ = r.Store.SetCandidateDecision(ctx, fingerprints, "failed", r.now())
		return err
	}
	feedPaths, err := publication.MaterializeFeeds(r.PublicRoot, pending.Posts, at)
	if err != nil {
		_ = r.Store.SetCandidateDecision(ctx, fingerprints, "failed", r.now())
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
		_ = r.Store.SetCandidateDecision(ctx, fingerprints, "pending", r.now())
		return err
	}
	if err := r.Store.SetCandidateDecision(ctx, fingerprints, "published", r.now()); err != nil {
		return err
	}
	if err := r.writeHealth(func(health *telemetry.Health) {
		health.LastConfirmedPush = timePointer(r.now())
		health.PausedReason = ""
	}); err != nil {
		return err
	}
	evidenceLinks := make([]string, 0, len(decision.ContentCandidate.Sources))
	for _, source := range decision.ContentCandidate.Sources {
		evidenceLinks = append(evidenceLinks, source.URL)
	}
	relatedIDs := append([]string(nil), decision.ContentCandidate.RelatedIDs...)
	if decision.ContentCandidate.CorrectionOf != "" {
		relatedIDs = append(relatedIDs, decision.ContentCandidate.CorrectionOf)
	}
	kind := memory.KindObservation
	switch decision.ContentCandidate.Type {
	case "prediction":
		kind = memory.KindPrediction
	case "question":
		kind = memory.KindOpenQuestion
	case "correction":
		kind = memory.KindCorrection
	}
	if err := memory.Write(ctx, r.Store, memory.Memory{
		ID: "post-" + postID, Kind: kind, Summary: decision.ContentCandidate.Title,
		EvidenceLinks: evidenceLinks, RelatedPostIDs: relatedIDs, UpdatedAt: at,
		SourceHash: fingerprintHash(r.ModelName, postID), CommitSHA: result.CommitSHA,
	}); err != nil {
		return fmt.Errorf("write confirmed publication memory: %w", err)
	}
	if r.Metrics != nil {
		r.Metrics.Published()
	}
	if r.Emitter != nil {
		r.Emitter.Emit(ctx, telemetry.EventPushConfirmed, postID, 0, "commit_sha", result.CommitSHA)
	}
	return nil
}

func buildRevisitQueue(posts []publication.PostRecord, items []observations.Candidate, now time.Time) []memory.RevisitSuggestion {
	inputs := make([]memory.RevisitInput, 0, len(posts))
	sourceText := make([]research.Evidence, 0, len(items))
	for _, item := range items {
		sourceText = append(sourceText, research.Evidence{SourceID: item.SourceID, Text: item.Headline + " " + item.Excerpt})
	}
	disputed := research.IndependentEvidenceCount(sourceText) < 2
	for _, post := range posts {
		if post.Type == "prediction" {
			inputs = append(inputs, memory.RevisitInput{PostID: post.ID, Title: post.Title, PredictionAt: post.PublishedAt, DueAt: post.PublishedAt.Add(30 * 24 * time.Hour), Disputed: disputed})
		}
	}
	return memory.BuildRevisitQueue(inputs, now)
}

func independentSourceCount(items []observations.Candidate) int {
	hosts := map[string]bool{}
	for _, item := range items {
		parsed, err := url.Parse(item.URL)
		if err == nil && parsed.Hostname() != "" {
			hosts[strings.ToLower(parsed.Hostname())] = true
		}
	}
	return len(hosts)
}

func fingerprintHash(modelName, postID string) string {
	digest := sha256.Sum256([]byte(modelName + "\n" + postID))
	return hex.EncodeToString(digest[:])
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

func (r Runtime) writeHealth(update func(*telemetry.Health)) error {
	if strings.TrimSpace(r.HealthPath) == "" {
		return nil
	}
	health := telemetry.EmptyHealth()
	if _, err := os.Stat(r.HealthPath); err == nil {
		existing, readErr := telemetry.ReadHealth(r.HealthPath)
		if readErr != nil {
			return readErr
		}
		health = existing
	}
	update(&health)
	return telemetry.WriteHealth(r.HealthPath, health)
}

func timePointer(value time.Time) *time.Time {
	value = value.UTC()
	return &value
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
