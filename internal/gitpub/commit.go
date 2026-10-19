package gitpub

import (
	"context"
	"errors"
	"strings"
)

func (r *Repository) Commit(ctx context.Context, postID string) (string, error) {
	if strings.TrimSpace(postID) == "" || strings.ContainsAny(postID, "\r\n") {
		return "", errors.New("post ID is required and must be single-line")
	}
	args := []string{
		"-c", "user.name=themachinewrites-bot", "-c", "user.email=bot@themachinewrites.com",
		"-c", "commit.gpgsign=false", "commit", "--no-gpg-sign", "-m", "publish:" + postID,
	}
	out, err := r.run(ctx, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}
