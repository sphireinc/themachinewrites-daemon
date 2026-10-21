package engine

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/editorial"
)

var rawHTMLPattern = regexp.MustCompile(`(?i)<\s*(script|iframe|object|embed|style|img|svg|math|link|meta)\b`)
var topicPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
var numericConfidencePattern = regexp.MustCompile(`(?i)(?:^|\s)[0-9]{1,3}\s*%`)

var (
	ErrMalformedProposal = errors.New("malformed codex proposal")
	ErrInvalidProposal   = errors.New("invalid codex proposal")
)

func ParseProposal(raw []byte) (Proposal, error) {
	return ParseProposalForMode(raw, string(editorial.Sources))
}

func ParseProposalForMode(raw []byte, selectedMode string) (Proposal, error) {
	if !utf8.Valid(raw) || len(raw) > 64*1024 {
		return Proposal{}, fmt.Errorf("%w: invalid UTF-8 or oversize output", ErrMalformedProposal)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var proposal Proposal
	if err := decoder.Decode(&proposal); err != nil {
		return Proposal{}, fmt.Errorf("%w: %v", ErrMalformedProposal, err)
	}
	if err := ensureEOF(decoder); err != nil {
		return Proposal{}, fmt.Errorf("%w: %v", ErrMalformedProposal, err)
	}
	proposal, err := ValidateProposalForMode(proposal, selectedMode)
	if err != nil {
		return Proposal{}, fmt.Errorf("%w: %v", ErrInvalidProposal, err)
	}
	return proposal, nil
}

func ensureEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("trailing JSON value")
		}
		return err
	}
	return nil
}

func ValidateProposalForMode(proposal Proposal, selectedMode string) (Proposal, error) {
	if len(proposal.Reason) < 1 || len(proposal.Reason) > 1000 {
		return Proposal{}, errors.New("reason must contain 1–1000 characters")
	}
	if hasDuplicates(proposal.Topics) {
		return Proposal{}, errors.New("topics must be unique")
	}
	if hasDuplicates(proposal.RelatedIDs) {
		return Proposal{}, errors.New("related_ids must be unique")
	}
	if hasDuplicates(proposal.RequestedIDs) {
		return Proposal{}, errors.New("requested_ids must be unique")
	}
	switch proposal.Action {
	case ActionDefer, ActionInvestigate:
		if proposal.Title != "" || proposal.Body != "" || len(proposal.Sources) != 0 || proposal.EditorialMode != "" || proposal.PrimaryTopic != "" || proposal.Factuality != "" || proposal.Prediction != nil || proposal.CorrectionError != "" || proposal.CorrectedInformation != "" {
			return Proposal{}, errors.New("nonpublish proposals must not contain publication fields")
		}
		return proposal, nil
	case ActionPublish:
		if len([]rune(proposal.Title)) < 5 || len([]rune(proposal.Title)) > 160 {
			return Proposal{}, errors.New("publish title must contain 5–160 characters")
		}
		mode, ok := editorial.Lookup(selectedMode)
		if !ok || mode.ID == editorial.Random {
			return Proposal{}, fmt.Errorf("selected editorial mode %q is invalid", selectedMode)
		}
		if proposal.EditorialMode == "" {
			proposal.EditorialMode = selectedMode
		}
		if proposal.Type == "correction" && proposal.EditorialMode == string(editorial.Corrections) {
			mode, _ = editorial.Lookup(string(editorial.Corrections))
		} else if proposal.EditorialMode != selectedMode {
			return Proposal{}, fmt.Errorf("proposal editorial_mode %q does not match selected mode %q", proposal.EditorialMode, selectedMode)
		}
		if selectedMode == string(editorial.Corrections) && proposal.Type != "correction" {
			return Proposal{}, errors.New("corrections mode may publish only corrections")
		}
		switch proposal.Type {
		case "observation", "analysis", "question", "prediction", "reflection", "correction":
		default:
			return Proposal{}, errors.New("unsupported publication type")
		}
		if len([]rune(proposal.Body)) < 1 || len([]rune(proposal.Body)) > 20000 {
			return Proposal{}, errors.New("publish body must contain 1–20000 characters")
		}
		if rawHTMLPattern.MatchString(proposal.Body) || strings.ContainsRune(proposal.Body, '\x00') {
			return Proposal{}, errors.New("publish body contains executable markup or control characters")
		}
		if len(proposal.Sources) > 20 {
			return Proposal{}, errors.New("publish proposal contains too many sources")
		}
		for _, source := range proposal.Sources {
			parsed, err := url.Parse(source.URL)
			if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" {
				return Proposal{}, errors.New("source URL must be absolute HTTP(S)")
			}
			if _, err := time.Parse(time.RFC3339Nano, source.RetrievedAt); err != nil {
				return Proposal{}, errors.New("source retrieved_at must be RFC3339")
			}
		}
		if len(proposal.Topics) < 1 || len(proposal.Topics) > 12 {
			return Proposal{}, errors.New("publish proposal requires 1–12 topics")
		}
		for _, topic := range proposal.Topics {
			if len(topic) > 48 || !topicPattern.MatchString(topic) {
				return Proposal{}, errors.New("topics must use lowercase extensible identifiers")
			}
		}
		if proposal.PrimaryTopic == "" {
			proposal.PrimaryTopic = proposal.Topics[0]
		}
		if len(proposal.PrimaryTopic) > 48 || !topicPattern.MatchString(proposal.PrimaryTopic) || !contains(proposal.Topics, proposal.PrimaryTopic) {
			return Proposal{}, errors.New("primary_topic must be one of topics")
		}
		if strings.TrimSpace(proposal.ConfidenceRationale) == "" {
			return Proposal{}, errors.New("publish proposal requires confidence rationale")
		}
		if len(proposal.Sources) == 0 {
			if mode.Evidence != editorial.ReflectiveEvidence || proposal.Factuality != "nonfactual_reflection" {
				return Proposal{}, errors.New("source-free publication is allowed only for explicitly nonfactual reflective modes")
			}
		} else {
			if proposal.Factuality == "" {
				proposal.Factuality = "source_supported"
			}
			if proposal.Factuality != "source_supported" {
				return Proposal{}, errors.New("a publication with citations must be marked source_supported")
			}
		}
		if proposal.Type == "prediction" {
			if proposal.Prediction == nil {
				return Proposal{}, errors.New("prediction publication requires structured prediction metadata")
			}
			if err := validatePrediction(*proposal.Prediction); err != nil {
				return Proposal{}, err
			}
		} else if proposal.Prediction != nil {
			return Proposal{}, errors.New("prediction metadata is only valid for prediction publications")
		}
		if proposal.CorrectionOf != "" {
			if proposal.Type != "correction" || !validCorrectionID(proposal.CorrectionOf) {
				return Proposal{}, errors.New("correction_of requires a correction type and valid ULID")
			}
			if strings.TrimSpace(proposal.CorrectionError) == "" || strings.TrimSpace(proposal.CorrectedInformation) == "" || len(proposal.Sources) == 0 {
				return Proposal{}, errors.New("correction requires the original error, corrected information, and supporting sources")
			}
		} else if proposal.Type == "correction" {
			return Proposal{}, errors.New("correction publication requires correction_of")
		} else if proposal.CorrectionError != "" || proposal.CorrectedInformation != "" {
			return Proposal{}, errors.New("correction details are only valid for correction publications")
		}
		if (mode.ID == editorial.FollowUp || mode.ID == editorial.Retrospective || mode.ID == editorial.SelfReview) && len(proposal.RelatedIDs) == 0 {
			return Proposal{}, fmt.Errorf("%s publication requires a reference to prior work", mode.ID)
		}
	default:
		return Proposal{}, errors.New("unsupported editorial action")
	}
	return proposal, nil
}

func validatePrediction(prediction Prediction) error {
	for name, value := range map[string]string{
		"outcome": prediction.Outcome, "reasoning": prediction.Reasoning,
		"horizon": prediction.Horizon, "uncertainty": prediction.Uncertainty,
	} {
		if strings.TrimSpace(value) == "" || len([]rune(value)) > 2000 {
			return fmt.Errorf("prediction %s must contain 1–2000 characters", name)
		}
	}
	if numericConfidencePattern.MatchString(prediction.Uncertainty) {
		return errors.New("prediction uncertainty must be qualitative; do not fabricate numerical confidence")
	}
	if len(prediction.InvalidationConditions) < 1 || len(prediction.InvalidationConditions) > 8 {
		return errors.New("prediction requires 1–8 invalidation conditions")
	}
	for _, condition := range prediction.InvalidationConditions {
		if strings.TrimSpace(condition) == "" || len([]rune(condition)) > 1000 {
			return errors.New("prediction invalidation conditions must each contain 1–1000 characters")
		}
	}
	return nil
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func hasDuplicates(values []string) bool {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			return true
		}
		seen[value] = struct{}{}
	}
	return false
}

func validCorrectionID(id string) bool {
	if len(id) != 26 || id[0] < '0' || id[0] > '7' {
		return false
	}
	for _, character := range id {
		if !strings.ContainsRune("0123456789ABCDEFGHJKMNPQRSTVWXYZ", character) {
			return false
		}
	}
	return true
}
