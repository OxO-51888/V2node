package task

import (
	"context"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
)

func init() {
	log.SetOutput(io.Discard)
}

func TestTimeoutPressureDoesNotStartOverlappingWorkers(t *testing.T) {
	release := make(chan struct{})
	var started int32
	tk := &Task{
		Name:     "stress",
		Interval: time.Millisecond,
		Timeout:  time.Millisecond,
		Execute: func(ctx context.Context) error {
			atomic.AddInt32(&started, 1)
			<-release
			return nil
		},
	}

	if err := tk.ExecuteWithTimeout(); err != nil {
		t.Fatalf("first ExecuteWithTimeout() error = %v", err)
	}
	if got := atomic.LoadInt32(&started); got != 1 {
		t.Fatalf("started after first timeout = %d, want 1", got)
	}

	var wg sync.WaitGroup
	for i := 0; i < 500; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				if err := tk.ExecuteWithTimeout(); err != nil {
					t.Errorf("ExecuteWithTimeout() error = %v", err)
				}
			}
		}()
	}
	wg.Wait()

	if got := atomic.LoadInt32(&started); got != 1 {
		t.Fatalf("started under timeout pressure = %d, want 1", got)
	}

	close(release)
}

func TestPanelReportTasksDoNotOverlapUnderHundredThousandRetries(t *testing.T) {
	names := []string{
		"reportUserTrafficTask/gm",
		"reportUserTrafficTask/nnm",
		"reportUserTrafficTask/ovo",
		"reportUserTrafficTask/yiyuan",
		"reportUserTrafficTask/clash",
		"reportUserTrafficTask/pianyi",
	}

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			release := make(chan struct{})
			var started int32
			tk := &Task{
				Name:     name,
				Interval: time.Millisecond,
				Timeout:  time.Millisecond,
				Execute: func(ctx context.Context) error {
					atomic.AddInt32(&started, 1)
					<-release
					return nil
				},
			}

			if err := tk.ExecuteWithTimeout(); err != nil {
				t.Fatalf("first ExecuteWithTimeout() error = %v", err)
			}

			const attempts = 100000
			var wg sync.WaitGroup
			for i := 0; i < 100; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for j := 0; j < attempts/100; j++ {
						if err := tk.ExecuteWithTimeout(); err != nil {
							t.Errorf("ExecuteWithTimeout() error = %v", err)
						}
					}
				}()
			}
			wg.Wait()

			if got := atomic.LoadInt32(&started); got != 1 {
				t.Fatalf("%s started %d worker(s) after %d retries, want 1", name, got, attempts)
			}

			close(release)
		})
	}
}
