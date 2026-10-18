package sources

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	ErrFeedTooLarge     = errors.New("feed exceeds maximum size")
	ErrTooManyRedirects = errors.New("feed redirect limit exceeded")
)

type Cursor struct {
	ETag         string
	LastModified string
}

type FetchResult struct {
	Body         []byte
	ETag         string
	LastModified string
	NotModified  bool
}

type Fetcher struct {
	Client       *http.Client
	MaxBytes     int64
	MaxRedirects int
	Timeout      time.Duration
	Validator    *URLValidator
}

func NewFetcher(validator *URLValidator) *Fetcher {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	return &Fetcher{
		Client:       &http.Client{Transport: transport},
		MaxBytes:     2 << 20,
		MaxRedirects: 3,
		Timeout:      15 * time.Second,
		Validator:    validator,
	}
}

func (f *Fetcher) Fetch(ctx context.Context, target *url.URL, cursor Cursor) (FetchResult, error) {
	if f == nil || f.Client == nil || f.Validator == nil {
		return FetchResult{}, errors.New("fetcher is not configured")
	}
	if err := f.Validator.Check(ctx, target); err != nil {
		return FetchResult{}, err
	}
	requestCtx := ctx
	var cancel context.CancelFunc
	if f.Timeout > 0 {
		requestCtx, cancel = context.WithTimeout(ctx, f.Timeout)
		defer cancel()
	}
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, target.String(), nil)
	if err != nil {
		return FetchResult{}, fmt.Errorf("create feed request: %w", err)
	}
	request.Header.Set("User-Agent", "TheMachineWrites/1.0 (+https://themachinewrites.com)")
	request.Header.Set("Accept", "application/atom+xml, application/rss+xml, application/xml;q=0.9, text/xml;q=0.8")
	if cursor.ETag != "" {
		request.Header.Set("If-None-Match", cursor.ETag)
	}
	if cursor.LastModified != "" {
		request.Header.Set("If-Modified-Since", cursor.LastModified)
	}

	client := *f.Client
	redirects := 0
	client.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		redirects++
		if redirects > f.MaxRedirects {
			return ErrTooManyRedirects
		}
		if strings.TrimSpace(next.URL.Scheme) == "" || next.URL.Hostname() == "" {
			return fmt.Errorf("invalid redirect URL")
		}
		return f.Validator.Check(next.Context(), next.URL)
	}
	response, err := client.Do(request)
	if err != nil {
		return FetchResult{}, fmt.Errorf("fetch feed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotModified {
		return FetchResult{NotModified: true}, nil
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return FetchResult{}, fmt.Errorf("fetch feed: unexpected HTTP status %d", response.StatusCode)
	}
	limited := io.LimitReader(response.Body, f.MaxBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return FetchResult{}, fmt.Errorf("read feed body: %w", err)
	}
	if int64(len(body)) > f.MaxBytes {
		return FetchResult{}, ErrFeedTooLarge
	}
	return FetchResult{
		Body:         body,
		ETag:         response.Header.Get("ETag"),
		LastModified: response.Header.Get("Last-Modified"),
	}, nil
}
