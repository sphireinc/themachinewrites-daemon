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
)

var rawHTMLPattern = regexp.MustCompile(`(?i)<\s*(script|iframe|object|embed|style|img|svg|math|link|meta)\b`)

var (
	ErrMalformedProposal = errors.New("malformed codex proposal")
	ErrInvalidProposal   = errors.New("invalid codex proposal")
)

func ParseProposal(raw []byte) (Proposal, error) {
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
	if err := validateProposal(proposal); err != nil {
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

func validateProposal(proposal Proposal) error {
	if len(proposal.Reason) < 1 || len(proposal.Reason) > 1000 {
		return errors.New("reason must contain 1–1000 characters")
	}
	if hasDuplicates(proposal.Topics) {
		return errors.New("topics must be unique")
	}
	if hasDuplicates(proposal.RelatedIDs) {
		return errors.New("related_ids must be unique")
	}
	if hasDuplicates(proposal.RequestedIDs) {
		return errors.New("requested_ids must be unique")
	}
	switch proposal.Action {
	case ActionDefer, ActionInvestigate:
		if proposal.Title != "" || proposal.Body != "" || len(proposal.Sources) != 0 {
			return errors.New("nonpublish proposals must not contain publication fields")
		}
		return nil
	case ActionPublish:
		if len([]rune(proposal.Title)) < 5 || len([]rune(proposal.Title)) > 160 {
			return errors.New("publish title must contain 5–160 characters")
		}
		switch proposal.Type {
		case "observation", "analysis", "question", "prediction", "reflection", "correction":
		default:
			return errors.New("unsupported publication type")
		}
		if len([]rune(proposal.Body)) < 1 || len([]rune(proposal.Body)) > 20000 {
			return errors.New("publish body must contain 1–20000 characters")
		}
		if rawHTMLPattern.MatchString(proposal.Body) || strings.ContainsRune(proposal.Body, '\x00') {
			return errors.New("publish body contains executable markup or control characters")
		}
		if len(proposal.Sources) < 1 || len(proposal.Sources) > 20 {
			return errors.New("publish proposal must contain sources")
		}
		for _, source := range proposal.Sources {
			parsed, err := url.Parse(source.URL)
			if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" {
				return errors.New("source URL must be absolute HTTP(S)")
			}
			if _, err := time.Parse(time.RFC3339Nano, source.RetrievedAt); err != nil {
				return errors.New("source retrieved_at must be RFC3339")
			}
		}
		if len(proposal.Topics) < 1 || len(proposal.Topics) > 12 {
			return errors.New("publish proposal requires 1–12 topics")
		}
		if strings.TrimSpace(proposal.ConfidenceRationale) == "" {
			return errors.New("publish proposal requires confidence rationale")
		}
		if proposal.CorrectionOf != "" {
			if proposal.Type != "correction" || !validCorrectionID(proposal.CorrectionOf) {
				return errors.New("correction_of requires a correction type and valid ULID")
			}
		} else if proposal.Type == "correction" {
			return errors.New("correction publication requires correction_of")
		}
	}
	return nil
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
