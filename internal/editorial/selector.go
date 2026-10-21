package editorial

import (
	cryptorand "crypto/rand"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"
)

var ErrNoEligibleMode = errors.New("no eligible weighted editorial mode")

type Candidate struct {
	Mode            Mode
	Eligible        bool
	Reason          string
	Topics          []string
	FreshnessFactor float64
	SignalFactor    float64
	SignalReason    string
}

type RecentSelection struct {
	Mode   ID
	Topics []string
}

type SelectionConfig struct {
	Weights            map[string]int
	ModeRepeatWindow   int
	TopicRepeatWindow  int
	ModeRepeatPenalty  float64
	TopicRepeatPenalty float64
}

type WeightedCandidate struct {
	Mode            Mode
	Reason          string
	Topics          []string
	BaseWeight      int
	ModePenalty     float64
	TopicPenalty    float64
	FreshnessFactor float64
	SignalFactor    float64
	SignalReason    string
	EffectiveWeight uint64
}

type Selection struct {
	Mode        *Mode
	Reason      string
	Eligible    []WeightedCandidate
	TotalWeight uint64
}

// RandomBelow is injectable so selection can be deterministic in tests.
type RandomBelow func(max uint64) (uint64, error)

func CryptoRandomBelow(max uint64) (uint64, error) {
	if max == 0 {
		return 0, errors.New("random bound must be positive")
	}
	value, err := cryptorand.Int(cryptorand.Reader, new(big.Int).SetUint64(max))
	if err != nil {
		return 0, fmt.Errorf("secure random selection: %w", err)
	}
	return value.Uint64(), nil
}

func Choose(candidates []Candidate, settings SelectionConfig, history []RecentSelection, randomBelow RandomBelow) (Selection, error) {
	if randomBelow == nil {
		randomBelow = CryptoRandomBelow
	}
	selection := Selection{Eligible: make([]WeightedCandidate, 0, len(candidates))}
	for _, candidate := range candidates {
		if !candidate.Eligible || !candidate.Mode.RandomEligible {
			continue
		}
		weight := settings.Weights[string(candidate.Mode.ID)]
		if weight <= 0 {
			continue
		}
		modePenalty := recentModePenalty(candidate.Mode.ID, settings, history)
		topicPenalty := recentTopicPenalty(candidate.Topics, settings, history)
		freshnessFactor := candidate.FreshnessFactor
		if freshnessFactor == 0 {
			freshnessFactor = 1
		}
		if math.IsNaN(freshnessFactor) || math.IsInf(freshnessFactor, 0) || freshnessFactor < 0 || freshnessFactor > 1 {
			return Selection{}, fmt.Errorf("invalid source freshness factor for %q", candidate.Mode.ID)
		}
		signalFactor := candidate.SignalFactor
		if signalFactor == 0 {
			signalFactor = 1
		}
		if math.IsNaN(signalFactor) || math.IsInf(signalFactor, 0) || signalFactor < 1 || signalFactor > 3 {
			return Selection{}, fmt.Errorf("invalid research signal factor for %q", candidate.Mode.ID)
		}
		effective := uint64(math.Round(float64(weight) * modePenalty * topicPenalty * freshnessFactor * signalFactor * 1_000_000))
		if effective == 0 {
			continue
		}
		selection.Eligible = append(selection.Eligible, WeightedCandidate{
			Mode: candidate.Mode, Reason: candidate.Reason, Topics: append([]string(nil), candidate.Topics...),
			BaseWeight: weight, ModePenalty: modePenalty, TopicPenalty: topicPenalty,
			FreshnessFactor: freshnessFactor, SignalFactor: signalFactor,
			SignalReason: candidate.SignalReason, EffectiveWeight: effective,
		})
		selection.TotalWeight += effective
	}
	if selection.TotalWeight == 0 {
		return selection, ErrNoEligibleMode
	}
	draw, err := randomBelow(selection.TotalWeight)
	if err != nil {
		return Selection{}, err
	}
	if draw >= selection.TotalWeight {
		return Selection{}, errors.New("random source returned value outside requested bound")
	}
	for _, candidate := range selection.Eligible {
		if draw < candidate.EffectiveWeight {
			mode := candidate.Mode
			selection.Mode = &mode
			selection.Reason = fmt.Sprintf("%s; base_weight=%d; mode_repeat_penalty=%.6g; topic_repeat_penalty=%.6g; source_freshness=%.6g; research_signal=%.6g (%s)", candidate.Reason, candidate.BaseWeight, candidate.ModePenalty, candidate.TopicPenalty, candidate.FreshnessFactor, candidate.SignalFactor, candidate.SignalReason)
			return selection, nil
		}
		draw -= candidate.EffectiveWeight
	}
	return Selection{}, errors.New("weighted selection exhausted its range")
}

func recentModePenalty(modeID ID, settings SelectionConfig, history []RecentSelection) float64 {
	if settings.ModeRepeatWindow <= 0 || settings.ModeRepeatPenalty >= 1 {
		return 1
	}
	count := 0
	for _, prior := range newest(history, settings.ModeRepeatWindow) {
		if prior.Mode == modeID {
			count++
		}
	}
	return math.Pow(settings.ModeRepeatPenalty, float64(count))
}

func recentTopicPenalty(topics []string, settings SelectionConfig, history []RecentSelection) float64 {
	if settings.TopicRepeatWindow <= 0 || settings.TopicRepeatPenalty >= 1 || len(topics) == 0 {
		return 1
	}
	seen := make(map[string]struct{})
	for _, prior := range newest(history, settings.TopicRepeatWindow) {
		for _, topic := range prior.Topics {
			seen[strings.ToLower(topic)] = struct{}{}
		}
	}
	count := 0
	counted := make(map[string]struct{})
	for _, topic := range topics {
		key := strings.ToLower(topic)
		if _, duplicate := counted[key]; duplicate {
			continue
		}
		counted[key] = struct{}{}
		if _, repeated := seen[key]; repeated {
			count++
		}
	}
	return math.Pow(settings.TopicRepeatPenalty, float64(count))
}

func newest(history []RecentSelection, limit int) []RecentSelection {
	if limit <= 0 || len(history) == 0 {
		return nil
	}
	if limit > len(history) {
		limit = len(history)
	}
	return history[len(history)-limit:]
}
