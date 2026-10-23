package conversation

import (
	"context"
	"errors"
	"time"
)

const outputTimeout = 5 * time.Second

var errOutputCongested = errors.New("conversation output remained congested")

// enqueueOutput preserves order while allowing a brief producer burst to drain.
// The producer owns cancellation; abandoning one turn must not close its socket.
func enqueueOutput(ctx context.Context, out chan<- Event, event Event, timeout time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case out <- event:
		return nil
	default:
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case out <- event:
		return nil
	case <-timer.C:
		return errOutputCongested
	}
}
