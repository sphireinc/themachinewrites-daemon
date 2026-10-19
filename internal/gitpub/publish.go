package gitpub

import (
	"context"
	"errors"
	"strings"
)

type Publisher interface {
	StageExact(context.Context, []string) error
	Commit(context.Context, string) (string, error)
	PushFastForward(context.Context, int) (string, error)
	RecoverPost(context.Context, string) (string, error)
}

type Outbox interface {
	SavePending(context.Context, string) error
	Acknowledge(context.Context, string, string) error
}

type PublishRequest struct {
	PostID          string
	Paths           []string
	MaxPushAttempts int
}

type PublishResult struct{ CommitSHA string }

func Publish(ctx context.Context, publisher Publisher, outbox Outbox, request PublishRequest) (PublishResult, error) {
	if publisher == nil || outbox == nil || strings.TrimSpace(request.PostID) == "" || len(request.Paths) == 0 {
		return PublishResult{}, errors.New("publication request is incomplete")
	}
	if err := outbox.SavePending(ctx, request.PostID); err != nil {
		return PublishResult{}, err
	}
	if sha, err := publisher.RecoverPost(ctx, request.PostID); err == nil {
		if err := outbox.Acknowledge(ctx, request.PostID, sha); err != nil {
			return PublishResult{}, err
		}
		return PublishResult{CommitSHA: sha}, nil
	} else if !errors.Is(err, ErrPostNotPublished) {
		return PublishResult{}, err
	}
	if err := publisher.StageExact(ctx, request.Paths); err != nil {
		return PublishResult{}, err
	}
	if _, err := publisher.Commit(ctx, request.PostID); err != nil {
		return PublishResult{}, err
	}
	sha, err := publisher.PushFastForward(ctx, request.MaxPushAttempts)
	if err != nil {
		return PublishResult{}, err
	}
	if err := outbox.Acknowledge(ctx, request.PostID, sha); err != nil {
		return PublishResult{}, err
	}
	return PublishResult{CommitSHA: sha}, nil
}
