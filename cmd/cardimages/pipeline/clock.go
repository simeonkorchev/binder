package pipeline

import (
	"context"
	"fmt"
	"time"
)

// SystemClock is the production Clock. It is a value type with no state, so it
// is passed by value and its methods take value receivers.
type SystemClock struct{}

// Now reports the current wall-clock time.
func (SystemClock) Now() time.Time { return time.Now() }

// Sleep waits for d, or returns early with the context's error when the run is
// cancelled — a timer alone would hold shutdown for a full rate-limit slot.
func (SystemClock) Sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return fmt.Errorf("sleeping: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}
