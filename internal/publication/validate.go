package publication

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/engine"
)

func ValidateProposal(proposal engine.Proposal) error {
	if proposal.Action != engine.ActionPublish {
		return errors.New("only publish proposals can become canonical posts")
	}
	if !utf8.ValidString(proposal.Title) || len([]rune(proposal.Title)) < 5 || len([]rune(proposal.Title)) > 160 {
		return errors.New("title must contain 5–160 valid UTF-8 characters")
	}
	for _, r := range proposal.Title {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r) || r == '<' || r == '>' {
			return errors.New("title must be single-line plain text without formatting controls or angle brackets")
		}
	}
	if strings.TrimSpace(proposal.Title) == "" {
		return errors.New("title is required")
	}
	if !utf8.ValidString(proposal.Body) || strings.TrimSpace(proposal.Body) == "" {
		return errors.New("body must be nonempty valid UTF-8 Markdown")
	}
	if len([]rune(proposal.Body)) > 20000 {
		return errors.New("body exceeds 20000 characters")
	}
	if strings.ContainsRune(proposal.Body, '\x00') {
		return fmt.Errorf("body contains NUL")
	}
	return nil
}

func ValidateSchemaVersion(version int) error {
	if version != 1 {
		return fmt.Errorf("unsupported post schema version %d; expected 1", version)
	}
	return nil
}
