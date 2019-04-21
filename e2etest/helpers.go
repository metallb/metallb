package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func waitFor(ctx context.Context, t *testing.T, test func() error) {
	t.Helper()
	var (
		lastErr = errors.New("dummy error")
		count   = 1
	)

	for {
		select {
		case <-ctx.Done():
			if count > 1 {
				t.Log("  (last message repeasted", count, "times")
			}
			t.Fatalf("timed out")
		default:
		}

		err := test()
		if err == nil {
			if count > 1 {
				t.Log("  (last message repeasted", count, "times")
			}
			return
		}

		if err.Error() != lastErr.Error() {
			if count > 1 {
				t.Log("  (last message repeated", count, "times")
			}
			t.Log(time.Now(), err)
			lastErr = err
			count = 1
		} else {
			count++
		}

		time.Sleep(100 * time.Millisecond)
	}
}
