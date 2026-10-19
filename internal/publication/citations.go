package publication

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/engine"
)

func ValidateCitations(proposal engine.Proposal) error {
	if len(proposal.Sources) == 0 {
		return errors.New("publish proposal requires at least one source")
	}
	seen := make(map[string]struct{}, len(proposal.Sources))
	for index, source := range proposal.Sources {
		if strings.TrimSpace(source.Title) == "" {
			return fmt.Errorf("source %d title is required", index+1)
		}
		parsed, err := url.Parse(source.URL)
		if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" {
			return fmt.Errorf("source %d URL must be absolute HTTPS", index+1)
		}
		if parsed.User != nil {
			return fmt.Errorf("source %d URL contains credentials", index+1)
		}
		if ip := net.ParseIP(parsed.Hostname()); ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()) {
			return fmt.Errorf("source %d URL uses a private or metadata address", index+1)
		}
		if strings.EqualFold(parsed.Hostname(), "localhost") || strings.EqualFold(parsed.Hostname(), "169.254.169.254") {
			return fmt.Errorf("source %d URL uses a local or metadata host", index+1)
		}
		if _, err := time.Parse(time.RFC3339Nano, source.RetrievedAt); err != nil {
			return fmt.Errorf("source %d retrieved_at must be RFC3339", index+1)
		}
		key := strings.ToLower(parsed.Scheme + "://" + parsed.Host + parsed.EscapedPath())
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf("source %d duplicates an earlier citation", index+1)
		}
		seen[key] = struct{}{}
	}
	return nil
}
