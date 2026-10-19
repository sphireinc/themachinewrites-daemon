package gitpub

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/publication"
)

var ErrPostNotPublished = errors.New("post not found in publication history")

func (r *Repository) RecoverPost(ctx context.Context, postID string) (string, error) {
	if len(postID) != 26 || strings.ContainsAny(postID, "\r\n/") {
		return "", errors.New("invalid post ID")
	}
	out, err := r.run(ctx, "log", "--all", "--format=%H", "--", "posts")
	if err != nil {
		return "", err
	}
	fields := strings.Fields(out)
	if len(fields) == 0 {
		return "", ErrPostNotPublished
	}
	for _, sha := range fields {
		canonical, err := publication.CanonicalPath(postID, publicationTimeFromULID(postID))
		if err != nil {
			return "", err
		}
		if _, err := r.run(ctx, "show", sha+":"+canonical); err != nil {
			continue
		}
		if !strings.HasPrefix(canonical, "posts/") {
			return "", fmt.Errorf("noncanonical recovery path")
		}
		return sha, nil
	}
	return "", ErrPostNotPublished
}

func publicationTimeFromULID(id string) time.Time {
	const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	var value uint64
	for _, character := range id[:10] {
		value = value*32 + uint64(strings.IndexRune(alphabet, character))
	}
	return time.UnixMilli(int64(value)).UTC()
}
