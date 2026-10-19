package gitpub

import (
	"context"
	"errors"
	"testing"
)

type fakePublisher struct {
	staged    []string
	committed bool
	pushed    bool
}

func (f *fakePublisher) StageExact(_ context.Context, paths []string) error {
	f.staged = append([]string(nil), paths...)
	return nil
}
func (f *fakePublisher) Commit(_ context.Context, _ string) (string, error) {
	f.committed = true
	return "commit-sha", nil
}
func (f *fakePublisher) PushFastForward(_ context.Context, _ int) (string, error) {
	f.pushed = true
	return "commit-sha", nil
}
func (f *fakePublisher) RecoverPost(_ context.Context, _ string) (string, error) {
	return "", ErrPostNotPublished
}

type fakeOutbox struct {
	pending  bool
	ackedSHA string
	acked    bool
}

func (f *fakeOutbox) SavePending(context.Context, string) error { f.pending = true; return nil }
func (f *fakeOutbox) Acknowledge(_ context.Context, _ string, sha string) error {
	f.acked = true
	f.ackedSHA = sha
	return nil
}

func TestPublishPersistsThenCommitsFeedsAndPushes(t *testing.T) {
	publisher := &fakePublisher{}
	outbox := &fakeOutbox{}
	result, err := Publish(context.Background(), publisher, outbox, PublishRequest{
		PostID: "post-one", Paths: []string{"feed/latest.json", "posts/2026/10/18/post-one.md"}, MaxPushAttempts: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !outbox.pending || !publisher.committed || !publisher.pushed || !outbox.acked || result.CommitSHA != "commit-sha" {
		t.Fatalf("publish state = result %#v, publisher %#v, outbox %#v", result, publisher, outbox)
	}
}

func TestPublishAbortsBeforeCommitWhenStageFails(t *testing.T) {
	publisher := &failingPublisher{}
	outbox := &fakeOutbox{}
	_, err := Publish(context.Background(), publisher, outbox, PublishRequest{PostID: "post-one", Paths: []string{"feed/latest.json"}, MaxPushAttempts: 2})
	if err == nil || publisher.committed || publisher.pushed {
		t.Fatalf("publish failure = %v, publisher=%#v", err, publisher)
	}
}

type failingPublisher struct{ fakePublisher }

func (f *failingPublisher) StageExact(context.Context, []string) error {
	return errors.New("stage failed")
}

type crashAfterPushPublisher struct{ fakePublisher }

func (f *crashAfterPushPublisher) RecoverPost(ctx context.Context, postID string) (string, error) {
	if f.pushed {
		return "commit-sha", nil
	}
	return f.fakePublisher.RecoverPost(ctx, postID)
}

type failFirstAckOutbox struct {
	fakeOutbox
	failed bool
}

func (f *failFirstAckOutbox) Acknowledge(ctx context.Context, postID, sha string) error {
	if !f.failed {
		f.failed = true
		return errors.New("simulated crash after push")
	}
	return f.fakeOutbox.Acknowledge(ctx, postID, sha)
}

func TestPublishRecoversAfterPushBeforeAckWithoutDuplicateCommit(t *testing.T) {
	publisher := &crashAfterPushPublisher{}
	outbox := &failFirstAckOutbox{}
	request := PublishRequest{PostID: "post-one", Paths: []string{"posts/one.md", "feed/latest.json"}, MaxPushAttempts: 2}
	if _, err := Publish(context.Background(), publisher, outbox, request); err == nil {
		t.Fatal("simulated post-push failure not returned")
	}
	publisher.committed = false
	result, err := Publish(context.Background(), publisher, outbox, request)
	if err != nil {
		t.Fatal(err)
	}
	if result.CommitSHA != "commit-sha" || publisher.committed || !outbox.acked {
		t.Fatalf("recovery result=%#v publisher=%#v outbox=%#v", result, publisher, outbox)
	}
}
