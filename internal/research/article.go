package research

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/sources"
	"golang.org/x/net/html"
)

var (
	ErrPolicyDenied       = errors.New("article fetch denied by robots or terms policy")
	ErrUnsupportedContent = errors.New("article content type is not permitted")
	ErrArticleTooLarge    = errors.New("article exceeds maximum size")
)

type FetchPolicy interface {
	Allow(context.Context, *url.URL) error
}

type FetchPolicyFunc func(context.Context, *url.URL) error

func (f FetchPolicyFunc) Allow(ctx context.Context, target *url.URL) error { return f(ctx, target) }

type Article struct {
	URL         string    `json:"url"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Text        string    `json:"text"`
	RetrievedAt time.Time `json:"retrieved_at"`
	ContentType string    `json:"content_type"`
}

type Fetcher struct {
	Client       *http.Client
	Validator    *sources.URLValidator
	Policy       FetchPolicy
	MaxBytes     int64
	MaxRedirects int
	Timeout      time.Duration
	MaxTextRunes int
	Now          func() time.Time
}

func NewFetcher(validator *sources.URLValidator, policy FetchPolicy) *Fetcher {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if validator != nil {
		transport.DialContext = validator.DialContext
		transport.TLSClientConfig = validator.TLSConfig("")
	}
	return &Fetcher{
		Client: &http.Client{Transport: transport}, Validator: validator, Policy: policy,
		MaxBytes: 512 * 1024, MaxRedirects: 3, Timeout: 10 * time.Second, MaxTextRunes: 12000,
		Now: time.Now,
	}
}

func (f *Fetcher) Fetch(ctx context.Context, target *url.URL) (Article, error) {
	if f == nil || f.Client == nil || f.Validator == nil || f.Policy == nil || target == nil {
		return Article{}, errors.New("article fetcher is not configured")
	}
	if err := f.Validator.Check(ctx, target); err != nil {
		return Article{}, err
	}
	if err := f.Policy.Allow(ctx, target); err != nil {
		return Article{}, fmt.Errorf("%w: %v", ErrPolicyDenied, err)
	}
	requestCtx := ctx
	if f.Timeout > 0 {
		var cancel context.CancelFunc
		requestCtx, cancel = context.WithTimeout(ctx, f.Timeout)
		defer cancel()
	}
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, target.String(), nil)
	if err != nil {
		return Article{}, fmt.Errorf("create article request: %w", err)
	}
	request.Header.Set("User-Agent", "TheMachineWrites/1.0 (+https://themachinewrites.com)")
	request.Header.Set("Accept", "text/html, application/xhtml+xml, text/plain;q=0.9")
	client := *f.Client
	redirects := 0
	client.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		redirects++
		if redirects > f.MaxRedirects {
			return sources.ErrTooManyRedirects
		}
		if err := f.Validator.Check(next.Context(), next.URL); err != nil {
			return err
		}
		target := next.URL
		return f.Policy.Allow(next.Context(), target)
	}
	response, err := client.Do(request)
	if err != nil {
		return Article{}, fmt.Errorf("fetch article: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return Article{}, fmt.Errorf("fetch article: unexpected HTTP status %d", response.StatusCode)
	}
	contentType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || !allowedContentType(contentType) {
		return Article{}, ErrUnsupportedContent
	}
	maxBytes := f.MaxBytes
	if maxBytes <= 0 || maxBytes > 2<<20 {
		maxBytes = 512 * 1024
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
	if err != nil {
		return Article{}, fmt.Errorf("read article body: %w", err)
	}
	if int64(len(body)) > maxBytes {
		return Article{}, ErrArticleTooLarge
	}
	if !utf8.Valid(body) {
		return Article{}, errors.New("article body is not UTF-8")
	}
	article := Article{URL: target.String(), ContentType: contentType, RetrievedAt: f.now()}
	if contentType == "text/plain" {
		article.Text = truncatePlain(string(body), f.maxTextRunes())
		return article, nil
	}
	article.Title, article.Description, article.Text = extractHTML(body, f.maxTextRunes())
	if strings.TrimSpace(article.Text) == "" {
		return Article{}, errors.New("article contains no readable text")
	}
	return article, nil
}

func (f *Fetcher) now() time.Time {
	if f.Now == nil {
		return time.Now().UTC()
	}
	return f.Now().UTC()
}

func (f *Fetcher) maxTextRunes() int {
	if f.MaxTextRunes < 1 || f.MaxTextRunes > 50000 {
		return 12000
	}
	return f.MaxTextRunes
}

func allowedContentType(contentType string) bool {
	switch strings.ToLower(contentType) {
	case "text/html", "application/xhtml+xml", "text/plain":
		return true
	default:
		return false
	}
}

func extractHTML(content []byte, maxRunes int) (string, string, string) {
	tokenizer := html.NewTokenizer(strings.NewReader(string(content)))
	skip := 0
	title, description := "", ""
	var text strings.Builder
	inTitle := false
	for {
		switch tokenizer.Next() {
		case html.ErrorToken:
			return strings.TrimSpace(title), strings.TrimSpace(description), truncatePlain(text.String(), maxRunes)
		case html.StartTagToken:
			token := tokenizer.Token()
			switch token.Data {
			case "script", "style", "noscript", "svg", "math", "iframe", "object", "embed", "template":
				skip++
			case "title":
				if skip == 0 {
					inTitle = true
				}
			case "meta":
				if description == "" {
					for index, attribute := range token.Attr {
						if attribute.Key == "name" && strings.EqualFold(attribute.Val, "description") && index+1 < len(token.Attr) && token.Attr[index+1].Key == "content" {
							description = token.Attr[index+1].Val
						}
					}
				}
			}
		case html.EndTagToken:
			name := tokenizer.Token().Data
			switch name {
			case "script", "style", "noscript", "svg", "math", "iframe", "object", "embed", "template":
				if skip > 0 {
					skip--
				}
			case "title":
				inTitle = false
			}
		case html.TextToken:
			if skip == 0 {
				value := strings.Join(strings.Fields(string(tokenizer.Text())), " ")
				if value != "" {
					if inTitle && title == "" {
						title = value
					} else {
						text.WriteString(value)
						text.WriteByte(' ')
					}
				}
			}
		}
	}
}

func truncatePlain(value string, maxRunes int) string {
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) > maxRunes {
		runes = runes[:maxRunes]
	}
	return string(runes)
}
