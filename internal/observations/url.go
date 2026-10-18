package observations

import (
	"errors"
	"net/url"
	"strings"
)

var trackingParameters = map[string]struct{}{
	"utm_source":   {},
	"utm_medium":   {},
	"utm_campaign": {},
	"utm_term":     {},
	"utm_content":  {},
	"gclid":        {},
	"fbclid":       {},
	"mc_cid":       {},
	"mc_eid":       {},
}

func CanonicalizeURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" {
		return "", errors.New("citation URL must be absolute HTTP(S)")
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Hostname())
	if port := parsed.Port(); port != "" && port != "80" && port != "443" {
		parsed.Host += ":" + port
	}
	parsed.User = nil
	parsed.Fragment = ""
	parsed.RawFragment = ""
	query := parsed.Query()
	for key := range query {
		if _, tracking := trackingParameters[strings.ToLower(key)]; tracking {
			query.Del(key)
		}
	}
	parsed.RawQuery = query.Encode()
	parsed.Path = strings.TrimSuffix(parsed.Path, "/")
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	return parsed.String(), nil
}
