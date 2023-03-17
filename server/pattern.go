package server

import "sync"

type Global[T any] struct {
	data T
	once sync.Once
}

func (s *Global[T]) Init(initValue T) {
	s.once.Do(func() {
		s.data = initValue
	})
}

func (s *Global[T]) Value() T {
	return s.data
}
