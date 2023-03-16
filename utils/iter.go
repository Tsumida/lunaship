package utils

import "github.com/pkg/errors"

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
