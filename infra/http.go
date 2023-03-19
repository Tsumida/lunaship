package infra

import "github.com/samber/mo"

type HttpResult[T any] struct {
	mo.Result[T]
	isNetworkErr bool
}

//go:inline
func (r *HttpResult[T]) IsNetworkErr() bool {
	return r.isNetworkErr
}

// Record req, resp and raw http body for reasoning.
func InvokeExternalAPI[T any]() *HttpResult[T] {
	panic("todo")
}
