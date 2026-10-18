package sources

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

var ErrUnsafeURL = errors.New("unsafe outbound URL")

type Resolver func(context.Context, string) ([]net.IP, error)

type URLValidator struct {
	Resolve            Resolver
	AllowLocalFixtures bool
}

func NewURLValidator(resolve Resolver, allowLocalFixtures bool) *URLValidator {
	if resolve == nil {
		resolve = func(ctx context.Context, host string) ([]net.IP, error) {
			return net.DefaultResolver.LookupIP(ctx, "ip", host)
		}
	}
	return &URLValidator{Resolve: resolve, AllowLocalFixtures: allowLocalFixtures}
}

func (v *URLValidator) Check(ctx context.Context, parsed *url.URL) error {
	if v == nil || parsed == nil || parsed.Hostname() == "" {
		return fmt.Errorf("%w: URL host is required", ErrUnsafeURL)
	}
	localHost := parsed.Hostname() == "localhost" || parsed.Hostname() == "127.0.0.1" || parsed.Hostname() == "::1"
	allowedScheme := parsed.Scheme == "https" || (v.AllowLocalFixtures && parsed.Scheme == "http" && localHost)
	if !allowedScheme {
		return fmt.Errorf("%w: scheme/host must use HTTPS%s", ErrUnsafeURL, localHTTPHint(v.AllowLocalFixtures))
	}
	if parsed.User != nil {
		return fmt.Errorf("%w: URL must not include credentials", ErrUnsafeURL)
	}
	if v.AllowLocalFixtures && parsed.Scheme == "http" && localHost {
		return nil
	}
	addresses, err := v.Resolve(ctx, parsed.Hostname())
	if err != nil {
		return fmt.Errorf("%w: resolve host: %v", ErrUnsafeURL, err)
	}
	if len(addresses) == 0 {
		return fmt.Errorf("%w: host resolved to no addresses", ErrUnsafeURL)
	}
	for _, address := range addresses {
		if unsafeIP(address) {
			return fmt.Errorf("%w: host resolves to a private or special-use address", ErrUnsafeURL)
		}
	}
	return nil
}

func localHTTPHint(allow bool) string {
	if allow {
		return "; HTTP is limited to localhost test fixtures"
	}
	return ""
}

func unsafeIP(ip net.IP) bool {
	return ip == nil ||
		ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified() ||
		strings.EqualFold(ip.String(), "169.254.169.254")
}
