package task

import (
	"context"
	"errors"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type Task struct {
	Name            string
	Interval        time.Duration
	Execute         func(context.Context) error
	Access          sync.RWMutex
	ExecuteLock     sync.Mutex
	Running         bool
	ReloadCh        chan struct{}
	ReloadOnTimeout bool
	Timeout         time.Duration
	Stop            chan struct{}
}

func (t *Task) Start(first bool) error {
	t.Access.Lock()
	if t.Running {
		t.Access.Unlock()
		return nil
	}
	t.Running = true
	t.Stop = make(chan struct{})
	t.Access.Unlock()
	go func() {
		if first {
			if err := t.ExecuteWithTimeout(); err != nil {
				return
			}
		}

		timer := time.NewTimer(t.currentInterval())
		defer timer.Stop()
		for {
			select {
			case <-timer.C:
				// continue
			case <-t.Stop:
				return
			}

			if err := t.ExecuteWithTimeout(); err != nil {
				log.Errorf("Task %s execution error: %v", t.Name, err)
				return
			}
			timer.Reset(t.currentInterval())
		}
	}()

	return nil
}

func (t *Task) currentInterval() time.Duration {
	t.Access.RLock()
	defer t.Access.RUnlock()
	return t.Interval
}

func (t *Task) UpdateInterval(interval time.Duration) {
	if interval <= 0 {
		return
	}
	t.Access.Lock()
	oldInterval := t.Interval
	t.Interval = interval
	t.Access.Unlock()
	if oldInterval != interval {
		log.Infof("Task %s interval updated from %s to %s", t.Name, oldInterval, interval)
	}
}

func (t *Task) ExecuteWithTimeout() error {
	if !t.ExecuteLock.TryLock() {
		log.Warningf("Task %s previous execution still running, skip this interval", t.Name)
		return nil
	}
	var unlockOnce sync.Once
	unlock := func() {
		unlockOnce.Do(func() {
			t.ExecuteLock.Unlock()
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), t.currentTimeout())
	defer cancel()
	done := make(chan error, 1)

	go func() {
		defer unlock()
		done <- t.Execute(ctx)
	}()

	select {
	case <-ctx.Done():
		unlock()
		if t.ReloadOnTimeout && t.ReloadCh != nil {
			log.Errorf("Task %s execution timed out, reloading", t.Name)
			select {
			case t.ReloadCh <- struct{}{}:
			default:
			}
		} else {
			log.Errorf("Task %s execution timed out, will retry on next interval", t.Name)
		}
		return nil
	case err := <-done:
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil
		}
		return err
	}
}

func (t *Task) currentTimeout() time.Duration {
	t.Access.RLock()
	timeout := t.Timeout
	interval := t.Interval
	t.Access.RUnlock()
	if timeout > 0 {
		return timeout
	}
	return min(5*interval, 5*time.Minute)
}

func (t *Task) safeStop() {
	t.Access.Lock()
	if t.Running {
		t.Running = false
		close(t.Stop)
	}
	t.Access.Unlock()
}

func (t *Task) Close() {
	t.safeStop()
	log.Warningf("Task %s stopped", t.Name)
}
