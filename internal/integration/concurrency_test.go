package integration

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/supervisor"
)

func TestConcurrentPublisherContentionAllowsOneLeaseOnly(t *testing.T) {
	root := filepath.Join(t.TempDir(), "publishing-lease")
	start := make(chan struct{})
	type outcome struct {
		lease *supervisor.Lease
		err   error
	}
	results := make(chan outcome, 2)
	var workers sync.WaitGroup
	for _, worker := range []string{"daemon-a", "daemon-b"} {
		workers.Add(1)
		go func(workerID string) {
			defer workers.Done()
			<-start
			lease, err := supervisor.AcquireLease(root, workerID)
			results <- outcome{lease: lease, err: err}
		}(worker)
	}
	close(start)
	workers.Wait()
	close(results)

	acquired := 0
	var held *supervisor.Lease
	for result := range results {
		if result.err == nil {
			acquired++
			held = result.lease
		}
	}
	if acquired != 1 || held == nil {
		t.Fatalf("concurrent lease acquisitions = %d", acquired)
	}
	if err := held.Release(); err != nil {
		t.Fatal(err)
	}
	next, err := supervisor.AcquireLease(root, "daemon-c")
	if err != nil {
		t.Fatalf("lease not available after release: %v", err)
	}
	if err := next.Release(); err != nil {
		t.Fatal(err)
	}
}
