package utils

import (
	"time"

	"github.com/pkg/errors"
)

type ChainError struct {
	FnIndex uint
	Err     error
}

func (ce *ChainError) Error() string {
	if ce.Err == nil {
		return ""
	}
	return errors.Errorf("index=%d, err=%w", ce.FnIndex, ce.Err.Error()).Error()
}

func Chain(
	fnList ...func() error,
) error {
	for index, fn := range fnList {
		if err := fn(); err != nil {
			return &ChainError{
				FnIndex: uint(index),
				Err:     err,
			}
		}
	}

	return nil
}

func Retry(
	maxRetry uint,
	timeWait time.Duration,
	action func() error,
	errHint string,
) error {
	var err error = action()
	cnt := uint(0)
	for err != nil && cnt < maxRetry {
		time.Sleep(timeWait)
		err = action()
		cnt += 1
	}
	return errors.WithMessagef(err, "exceed max-retry %d for %s", maxRetry, errHint)
}
