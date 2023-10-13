package fsm

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestDefaultDriver_Run(t *testing.T) {

	var (
		ctx        = context.Background()
		cnt uint64 = 0
		ch         = make(chan Event, 1)
	)

	d := &DefaultDriver{
		EventFetcher: func(ctx context.Context) <-chan Event {
			return (<-chan Event)(ch)
		},
		StopFn: func() bool {
			return atomic.LoadUint64(&cnt) > 10
		},
		SleepDur: 100 * time.Millisecond,
	}

	d.Run(ctx)

}
