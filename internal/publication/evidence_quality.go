package publication

import (
	"errors"
	"net/url"
	"regexp"
	"strings"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/engine"
)

var consequentialPattern = regexp.MustCompile(`(?i)\b(?:will|must|definitely|proves|proved|corrupt|fraud|guilty|responsible|caused|always|never)\b`)
var uncertaintyPattern = regexp.MustCompile(`(?i)\b(?:may|might|could|appears|suggests|uncertain|unclear|insufficient evidence|likely|possibly)\b`)

func ValidateEvidenceQuality(proposal engine.Proposal, independentSources int) error {
	if proposal.Action != engine.ActionPublish {
		return nil
	}
	if independentSources < 0 {
		return errors.New("independent source count cannot be negative")
	}
	text := proposal.Title + " " + proposal.Body
	if !consequentialPattern.MatchString(text) {
		return nil
	}
	if proposal.Type == "reflection" || proposal.Type == "question" {
		return nil
	}
	if independentSources >= 2 && len(proposal.Sources) >= 2 && uniqueCitationHosts(proposal.Sources) >= 2 {
		return nil
	}
	if proposal.Type == "prediction" && uncertaintyPattern.MatchString(text) {
		return nil
	}
	if !strings.Contains(strings.ToLower(proposal.Body), "uncertain") && !strings.Contains(strings.ToLower(proposal.Body), "insufficient evidence") {
		return errors.New("consequential claim requires independent corroboration or explicit uncertainty")
	}
	return nil
}

func uniqueCitationHosts(sources []engine.Source) int {
	hosts := map[string]bool{}
	for _, source := range sources {
		parsed, err := url.Parse(source.URL)
		if err == nil {
			hosts[strings.ToLower(parsed.Hostname())] = true
		}
	}
	return len(hosts)
}
