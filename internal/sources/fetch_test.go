package sources

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestFetcherSendsConditionalHeadersAndHandlesNotModified(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("If-None-Match") != `"etag-one"` {
			t.Errorf("If-None-Match = %q", request.Header.Get("If-None-Match"))
		}
		if request.Header.Get("If-Modified-Since") != "Sun, 18 Oct 2026 20:00:00 GMT" {
			t.Errorf("If-Modified-Since = %q", request.Header.Get("If-Modified-Since"))
		}
		return response(request, http.StatusNotModified, ""), nil
	})}
	fetcher := testFetcher(client)

	result, err := fetcher.Fetch(context.Background(), mustURL(t, "https://example.org/feed.xml"), Cursor{
		ETag:         `"etag-one"`,
		LastModified: "Sun, 18 Oct 2026 20:00:00 GMT",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.NotModified || result.Body != nil {
		t.Fatalf("Fetch() = %#v, want not modified without body", result)
	}
}

func TestFetcherRejectsOversizeTimeoutAndTooManyRedirects(t *testing.T) {
	t.Run("oversize", func(t *testing.T) {
		client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return response(request, http.StatusOK, strings.Repeat("x", 129)), nil
		})}
		fetcher := testFetcher(client)
		_, err := fetcher.Fetch(context.Background(), mustURL(t, "https://example.org/feed.xml"), Cursor{})
		if !errors.Is(err, ErrFeedTooLarge) {
			t.Fatalf("Fetch() error = %v, want ErrFeedTooLarge", err)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			<-request.Context().Done()
			return nil, request.Context().Err()
		})}
		fetcher := testFetcher(client)
		fetcher.Timeout = 25 * time.Millisecond
		_, err := fetcher.Fetch(context.Background(), mustURL(t, "https://example.org/feed.xml"), Cursor{})
		if err == nil || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Fetch() error = %v, want timeout", err)
		}
	})

	t.Run("redirect bound", func(t *testing.T) {
		var mu sync.Mutex
		requests := 0
		client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			mu.Lock()
			requests++
			mu.Unlock()
			next := *request
			next.URL = mustURL(t, "https://example.org/again")
			return &http.Response{
				StatusCode: http.StatusFound,
				Header:     http.Header{"Location": []string{next.URL.String()}},
				Body:       io.NopCloser(strings.NewReader("")),
				Request:    &next,
			}, nil
		})}
		fetcher := testFetcher(client)
		fetcher.MaxRedirects = 2
		_, err := fetcher.Fetch(context.Background(), mustURL(t, "https://example.org/feed.xml"), Cursor{})
		if !errors.Is(err, ErrTooManyRedirects) {
			t.Fatalf("Fetch() error = %v, want ErrTooManyRedirects", err)
		}
		if requests != 3 {
			t.Fatalf("requests = %d, want initial + two redirects", requests)
		}
	})
}

func TestRedirectValidatorBlocksPrivateTargets(t *testing.T) {
	productionValidator := NewURLValidator(func(context.Context, string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("127.0.0.1")}, nil
	}, false)
	err := productionValidator.Check(context.Background(), mustURL(t, "https://public.example/feed"))
	if !errors.Is(err, ErrUnsafeURL) {
		t.Fatalf("Check() error = %v, want ErrUnsafeURL", err)
	}

	for _, raw := range []string{
		"https://127.0.0.1/",
		"https://169.254.169.254/latest/meta-data",
		"https://10.0.0.1/",
	} {
		resolved := net.ParseIP(strings.TrimSuffix(strings.TrimPrefix(raw, "https://"), "/"))
		validator := NewURLValidator(func(context.Context, string) ([]net.IP, error) {
			return []net.IP{resolved}, nil
		}, false)
		if err := validator.Check(context.Background(), mustURL(t, raw)); !errors.Is(err, ErrUnsafeURL) {
			t.Errorf("Check(%q) error = %v, want ErrUnsafeURL", raw, err)
		}
	}
}

func testFetcher(client *http.Client) *Fetcher {
	return &Fetcher{
		Client:       client,
		MaxBytes:     128,
		MaxRedirects: 3,
		Timeout:      time.Second,
		Validator: NewURLValidator(func(context.Context, string) ([]net.IP, error) {
			return []net.IP{net.ParseIP("93.184.216.34")}, nil
		}, true),
	}
}

func response(request *http.Request, status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    request,
	}
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
