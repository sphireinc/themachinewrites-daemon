package daemon

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/editorial"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/engine"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/observations"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/publication"
	"github.com/TheMachineWrites/themachinewrites-daemon/internal/store"
)

type editorialCaptureEngine struct {
	inputs    []engine.Input
	proposals []engine.Proposal
}

func (e *editorialCaptureEngine) Run(_ context.Context, input engine.Input) (engine.Proposal, error) {
	e.inputs = append(e.inputs, input)
	if len(e.proposals) > 0 {
		proposal := e.proposals[0]
		e.proposals = e.proposals[1:]
		return proposal, nil
	}
	return engine.Proposal{Action: engine.ActionDefer, Reason: "No warranted publication."}, nil
}

func TestExplicitPhilosophyRunsWithoutPendingSourceCandidates(t *testing.T) {
	ctx := context.Background()
	state, err := store.Open(ctx, filepath.Join(t.TempDir(), "runtime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer state.Close()
	engineStub := &editorialCaptureEngine{}
	at := time.Date(2026, 10, 21, 14, 0, 0, 0, time.UTC)
	runtime := Runtime{
		Store: state, Engine: engineStub, PublicRoot: t.TempDir(), DryRun: true,
		EditorialMode: editorial.Philosophy,
		Now:           func() time.Time { return at },
	}
	if err := runtime.DecideAndPublish(ctx); err != nil {
		t.Fatal(err)
	}
	if len(engineStub.inputs) != 1 {
		t.Fatalf("engine calls = %d, want one source-independent editorial cycle", len(engineStub.inputs))
	}
	var bundle map[string]any
	if err := json.Unmarshal([]byte(engineStub.inputs[0].Bundle), &bundle); err != nil {
		t.Fatal(err)
	}
	if bundle["editorial_mode"] != "philosophy" {
		t.Fatalf("decision bundle editorial_mode = %#v", bundle["editorial_mode"])
	}
	history, err := state.RecentEditorialCycles(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || history[0].SelectedMode != "philosophy" || history[0].Outcome != "deferred" {
		t.Fatalf("editorial cycle history = %#v", history)
	}
}

func TestEveryEditorialModeReachesTheExistingDecisionEngine(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 10, 21, 14, 0, 0, 0, time.UTC)
	for _, mode := range editorial.IDs() {
		if mode == string(editorial.Random) {
			continue
		}
		t.Run(mode, func(t *testing.T) {
			state, err := store.Open(ctx, filepath.Join(t.TempDir(), "runtime.sqlite"))
			if err != nil {
				t.Fatal(err)
			}
			defer state.Close()
			for index, category := range []string{"science", "technology"} {
				_, err := observations.Ingest(ctx, state, observations.Candidate{
					URL: "https://example.org/" + category, Headline: "A verified development in " + category,
					SourceID: "fixture-source", SourceName: "Fixture Source", Category: category,
					PublishedAt: at.Add(-time.Duration(index) * time.Minute), RetrievedAt: at,
					Excerpt: "A fixture excerpt for eligibility testing.",
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			publicRoot := t.TempDir()
			priorID, err := publication.GenerateID("prior-publication", at.Add(-time.Hour))
			if err != nil {
				t.Fatal(err)
			}
			prior, err := publication.Render(engine.Proposal{
				Action: engine.ActionPublish, EditorialMode: string(editorial.Sources), PrimaryTopic: "science",
				Factuality: "source_supported", Title: "An earlier scientific observation", Type: "analysis",
				Body: "A prior fixture publication.", Topics: []string{"science"},
				Sources: []engine.Source{{Title: "Fixture evidence", URL: "https://example.org/evidence", RetrievedAt: at.Add(-time.Hour).Format(time.RFC3339)}},
			}, publication.RenderInput{ID: priorID, PublishedAt: at.Add(-time.Hour), ModelIdentifier: "fixture", PromptVersion: "fixture-v1", IndependentSources: 1})
			if err != nil {
				t.Fatal(err)
			}
			if err := publication.WriteRenderedPost(publicRoot, prior); err != nil {
				t.Fatal(err)
			}
			engineStub := &editorialCaptureEngine{}
			runtime := Runtime{
				Store: state, Engine: engineStub, PublicRoot: publicRoot, DryRun: true,
				EditorialMode: editorial.ID(mode), Now: func() time.Time { return at },
			}
			if err := runtime.DecideAndPublish(ctx); err != nil {
				t.Fatal(err)
			}
			if len(engineStub.inputs) != 1 || engineStub.inputs[0].EditorialMode != mode {
				t.Fatalf("decision engine received mode %q in %d calls; want %q in one call", func() string {
					if len(engineStub.inputs) == 0 {
						return ""
					}
					return engineStub.inputs[0].EditorialMode
				}(), len(engineStub.inputs), mode)
			}
			history, err := state.RecentEditorialCycles(ctx, 1)
			if err != nil || len(history) != 1 || history[0].SelectedMode != mode || history[0].Outcome != "deferred" {
				t.Fatalf("cycle result = %#v, %v; want selected %q and deferred", history, err, mode)
			}
			corpus, err := publication.LoadCorpus(publicRoot)
			if err != nil || len(corpus.Posts) != 1 || corpus.Posts[0].ID != priorID {
				t.Fatalf("dry-run/defer changed public corpus: %#v, %v", corpus.Posts, err)
			}
		})
	}
}

func TestPredictionReviewPersistsWithoutPublicationAndReturnsAsContext(t *testing.T) {
	ctx := context.Background()
	state, err := store.Open(ctx, filepath.Join(t.TempDir(), "runtime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer state.Close()
	now := time.Date(2026, 10, 21, 14, 0, 0, 0, time.UTC)
	predictionAt := now.Add(-40 * 24 * time.Hour)
	predictionID, err := publication.GenerateID("mature-prediction", predictionAt)
	if err != nil {
		t.Fatal(err)
	}
	publicRoot := t.TempDir()
	predictionPost, err := publication.Render(engine.Proposal{
		Action: engine.ActionPublish, EditorialMode: string(editorial.Predictions), PrimaryTopic: "technology", Factuality: "source_supported",
		Title: "A measurable forecast", Type: "prediction", Body: "A published forecast.", Topics: []string{"technology"},
		Sources:    []engine.Source{{Title: "Original evidence", URL: "https://example.org/original", RetrievedAt: predictionAt.Format(time.RFC3339)}},
		Prediction: &engine.Prediction{Outcome: "An event will occur", Reasoning: "Evidence points to the trend.", Horizon: "Within one month", Uncertainty: "The trend can change.", InvalidationConditions: []string{"The trend reverses."}},
	}, publication.RenderInput{ID: predictionID, PublishedAt: predictionAt, ModelIdentifier: "fixture", PromptVersion: "fixture-v1", IndependentSources: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := publication.WriteRenderedPost(publicRoot, predictionPost); err != nil {
		t.Fatal(err)
	}
	_, err = observations.Ingest(ctx, state, observations.Candidate{
		URL: "https://example.org/new-evidence", Headline: "New evidence for a prior forecast", SourceID: "fixture-source", SourceName: "Fixture Source",
		Category: "technology", PublishedAt: now.Add(-time.Hour), RetrievedAt: now, Excerpt: "New evidence available for evaluation.",
	})
	if err != nil {
		t.Fatal(err)
	}
	review := engine.PredictionReview{PredictionID: predictionID, Result: "partially_supported", Assessment: "The evidence supports part of the forecast, while its stated condition remains unresolved.",
		Sources: []engine.Source{{Title: "New evidence", URL: "https://example.org/new-evidence", RetrievedAt: now.Format(time.RFC3339)}}}
	engineStub := &editorialCaptureEngine{proposals: []engine.Proposal{
		{Action: engine.ActionDefer, Reason: "The review does not justify a new post.", PredictionReviews: []engine.PredictionReview{review}},
		{Action: engine.ActionDefer, Reason: "No new publication is warranted."},
	}}
	runtime := Runtime{Store: state, Engine: engineStub, PublicRoot: publicRoot, DryRun: true, EditorialMode: editorial.Questions, Now: func() time.Time { return now }}
	for range 2 {
		if err := runtime.DecideAndPublish(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if len(engineStub.inputs) != 2 {
		t.Fatalf("engine calls = %d, want two", len(engineStub.inputs))
	}
	var bundle map[string]any
	if err := json.Unmarshal([]byte(engineStub.inputs[1].Bundle), &bundle); err != nil {
		t.Fatal(err)
	}
	reviews, ok := bundle["prediction_review_history"].([]any)
	if !ok || len(reviews) != 1 || reviews[0].(map[string]any)["result"] != "partially_supported" {
		t.Fatalf("next-cycle prediction review context = %#v", bundle["prediction_review_history"])
	}
	priorPosts, ok := bundle["prior_publications"].([]any)
	if !ok || len(priorPosts) != 1 || priorPosts[0].(map[string]any)["id"] != predictionID {
		t.Fatalf("mature prediction absent from prior context: %#v", bundle["prior_publications"])
	}
	stored, err := state.RecentPredictionReviews(ctx, 10)
	if err != nil || len(stored) != 1 || stored[0].PredictionID != predictionID || stored[0].Result != "partially_supported" {
		t.Fatalf("stored prediction review = %#v, %v", stored, err)
	}
	corpus, err := publication.LoadCorpus(publicRoot)
	if err != nil || len(corpus.Posts) != 1 || corpus.Posts[0].ID != predictionID {
		t.Fatalf("review cycles changed public history: %#v, %v", corpus.Posts, err)
	}
}

func TestRandomEditorialModeIsResolvedAndRecordedForEachCycle(t *testing.T) {
	ctx := context.Background()
	state, err := store.Open(ctx, filepath.Join(t.TempDir(), "runtime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer state.Close()
	engineStub := &editorialCaptureEngine{}
	at := time.Date(2026, 10, 21, 14, 0, 0, 0, time.UTC)
	draws := 0
	runtime := Runtime{
		Store: state, Engine: engineStub, PublicRoot: t.TempDir(), DryRun: true,
		EditorialMode: editorial.Random,
		EditorialSettings: editorial.SelectionConfig{
			Weights: map[string]int{"philosophy": 1, "questions": 1}, ModeRepeatWindow: 1,
			ModeRepeatPenalty: 0,
		},
		RandomBelow: func(uint64) (uint64, error) { draws++; return 0, nil },
		Now:         func() time.Time { return at.Add(time.Duration(draws) * time.Millisecond) },
	}
	for range 2 {
		if err := runtime.DecideAndPublish(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if len(engineStub.inputs) != 2 || draws != 2 {
		t.Fatalf("engine calls/random draws = %d/%d, want 2/2", len(engineStub.inputs), draws)
	}
	var first, second map[string]any
	if err := json.Unmarshal([]byte(engineStub.inputs[0].Bundle), &first); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(engineStub.inputs[1].Bundle), &second); err != nil {
		t.Fatal(err)
	}
	if first["editorial_mode"] != "philosophy" || second["editorial_mode"] != "questions" {
		t.Fatalf("cycle modes = %v, %v; want philosophy then questions", first["editorial_mode"], second["editorial_mode"])
	}
}

func TestRecentSourceOnPublishedTopicMakesFollowUpModeEligible(t *testing.T) {
	posts := []publication.PostRecord{{ID: "01M589K0G00000000000000000", Topics: []string{"science"}}}
	items := []observations.Candidate{{Category: "science"}}
	runtime := Runtime{}
	availability := runtime.editorialAvailability(items, posts, nil, nil)
	if availability.FollowUpTargets != 1 {
		t.Fatalf("follow-up targets = %d, want one new-source match to a published topic", availability.FollowUpTargets)
	}
	hints := editorialTopicHints(items, posts, nil)
	if len(hints[editorial.FollowUp]) != 1 || hints[editorial.FollowUp][0] != "science" {
		t.Fatalf("follow-up topic hints = %v, want [science]", hints[editorial.FollowUp])
	}
}
