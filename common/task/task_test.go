package task

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestTimeoutDoesNotReloadByDefault(t *testing.T) {
	reloadCh := make(chan struct{}, 1)
	executed := make(chan struct{})
	release := make(chan struct{})
	tk := &Task{
		Name:     "test",
		Interval: 10 * time.Millisecond,
		ReloadCh: reloadCh,
		Execute: func(ctx context.Context) error {
			close(executed)
			<-release
			return nil
		},
	}

	if err := tk.ExecuteWithTimeout(); err != nil {
		t.Fatalf("ExecuteWithTimeout() error = %v", err)
	}
	close(release)

	select {
	case <-executed:
	default:
		t.Fatal("task did not execute")
	}
	select {
	case <-reloadCh:
		t.Fatal("timeout should not request reload by default")
	default:
	}
}

func TestSkipWhilePreviousExecutionIsStillRunning(t *testing.T) {
	release := make(chan struct{})
	var started int32
	tk := &Task{
		Name:     "test",
		Interval: 10 * time.Millisecond,
		Execute: func(ctx context.Context) error {
			atomic.AddInt32(&started, 1)
			<-release
			return nil
		},
	}

	if err := tk.ExecuteWithTimeout(); err != nil {
		t.Fatalf("first ExecuteWithTimeout() error = %v", err)
	}
	if err := tk.ExecuteWithTimeout(); err != nil {
		t.Fatalf("second ExecuteWithTimeout() error = %v", err)
	}
	close(release)

	if got := atomic.LoadInt32(&started); got != 1 {
		t.Fatalf("started = %d, want 1", got)
	}
}
