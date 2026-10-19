package research

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/sources"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func testFetcher(response *http.Response) *Fetcher {
	return &Fetcher{
		Client: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			response.Request = request
			return response, nil
		})},
		Validator: sources.NewURLValidator(func(context.Context, string) ([]net.IP, error) {
			return []net.IP{net.ParseIP("93.184.216.34")}, nil
		}, false),
		Policy:   FetchPolicyFunc(func(context.Context, *url.URL) error { return nil }),
		MaxBytes: 1024, MaxRedirects: 2, Timeout: time.Second, MaxTextRunes: 1000,
		Now: func() time.Time { return time.Unix(1, 0).UTC() },
	}
}

func response(contentType, body string) *http.Response {
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{contentType}}, Body: io.NopCloser(strings.NewReader(body))}
}

func target(t *testing.T) *url.URL {
	t.Helper()
	parsed, err := url.Parse("https://example.org/article")
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func TestArticleFetcherExtractsBoundedTextAndMetadata(t *testing.T) {
	fetcher := testFetcher(response("text/html; charset=utf-8", `<html><head><title>Fixture title</title><meta name="description" content="Fixture description"></head><body><script>bad()</script><p>A factual sentence.</p><p>More evidence.</p></body></html>`))
	article, err := fetcher.Fetch(context.Background(), target(t))
	if err != nil {
		t.Fatal(err)
	}
	if article.Title != "Fixture title" || article.Description != "Fixture description" || strings.Contains(article.Text, "bad()") || !strings.Contains(article.Text, "A factual sentence.") {
		t.Fatalf("article = %#v", article)
	}
	if article.URL != "https://example.org/article" || article.RetrievedAt.Unix() != 1 {
		t.Fatalf("provenance = %#v", article)
	}
}

func TestArticleFetcherRejectsPolicyBinaryPrivateRedirectAndOversize(t *testing.T) {
	t.Run("policy", func(t *testing.T) {
		fetcher := testFetcher(response("text/html", "<p>body</p>"))
		fetcher.Policy = FetchPolicyFunc(func(context.Context, *url.URL) error { return errors.New("robots disallow") })
		if _, err := fetcher.Fetch(context.Background(), target(t)); !errors.Is(err, ErrPolicyDenied) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("binary", func(t *testing.T) {
		fetcher := testFetcher(response("application/pdf", "pdf"))
		if _, err := fetcher.Fetch(context.Background(), target(t)); !errors.Is(err, ErrUnsupportedContent) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("oversize", func(t *testing.T) {
		fetcher := testFetcher(response("text/plain", strings.Repeat("x", 1025)))
		if _, err := fetcher.Fetch(context.Background(), target(t)); !errors.Is(err, ErrArticleTooLarge) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("redirect", func(t *testing.T) {
		fetcher := testFetcher(response("text/html", "<p>body</p>"))
		fetcher.Client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": []string{"https://private.example/article"}}, Body: io.NopCloser(strings.NewReader("")), Request: request}, nil
		})}
		fetcher.Validator = sources.NewURLValidator(func(_ context.Context, host string) ([]net.IP, error) {
			if host == "private.example" {
				return []net.IP{net.ParseIP("127.0.0.1")}, nil
			}
			return []net.IP{net.ParseIP("93.184.216.34")}, nil
		}, false)
		if _, err := fetcher.Fetch(context.Background(), target(t)); !errors.Is(err, sources.ErrUnsafeURL) {
			t.Fatalf("error = %v", err)
		}
	})
}
