package fsm

import (
	"context"
	"fmt"
	"time"
)

type Event interface {
	Resource
	EventSource() string
	EventReceiver() string
	EventType() string
	EventDesc() string
	Body() []byte
}

type DefaultDriver struct {
	EventFetcher func(ctx context.Context) <-chan Event
	StopFn       func() bool
	SleepDur     time.Duration
}

func NewDefaultDriver(stopFn func() bool) *DefaultDriver {
	return &DefaultDriver{
		StopFn:   stopFn,
		SleepDur: 50 * time.Millisecond,
	}
}
func (d *DefaultDriver) PrintLog(event Event) {
	fmt.Printf(
		"Got Event(id=%s, source=%s, target=%s):%s\n",
		event.ID(), event.EventSource(), event.EventReceiver(), string(event.Body()),
	)
}

func (d *DefaultDriver) Run(ctx context.Context) error {
	for d.StopFn() {
		select {
		case event, isOpened := <-d.EventFetcher(ctx):
			if !isOpened {
				break
			}
			d.PrintLog(event)
		default:
			time.Sleep(d.SleepDur)
		}
	}

	return nil
}
