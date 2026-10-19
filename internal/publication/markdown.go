package publication

import (
	"errors"
	"regexp"
	"strings"
)

var (
	rawTagPattern = regexp.MustCompile(`(?is)<\s*/?\s*[a-z][^>]*>`)
	eventPattern  = regexp.MustCompile(`(?i)\bon[a-z]+\s*=`)
	badScheme     = regexp.MustCompile(`(?i)(?:javascript|data|vbscript)\s*:`)
)

func ValidateMarkdown(body string) error {
	if strings.TrimSpace(body) == "" {
		return errors.New("Markdown body is empty")
	}
	if rawTagPattern.MatchString(body) || eventPattern.MatchString(body) || badScheme.MatchString(body) {
		return errors.New("Markdown contains raw HTML, event handlers or executable URL schemes")
	}
	if strings.HasPrefix(strings.TrimSpace(body), "---") {
		return errors.New("Markdown body cannot contain frontmatter delimiters")
	}
	return nil
}
