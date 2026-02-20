package main

import (
	"context"

	"connectrpc.com/connect"
	logsv1 "github.com/tsumida/lunaship/example/logs/gen"
	"github.com/tsumida/lunaship/log"
)

type DummyService struct{}

func NewDummyService() *DummyService {
	return &DummyService{}
}

func (s *DummyService) Ping(
	ctx context.Context,
	req *connect.Request[logsv1.PingRequest],
) (*connect.Response[logsv1.PingResponse], error) {
	message := req.Msg.GetMessage()
	if message == "" {
		message = "pong"
	}
	traceID, spanID, _ := log.TraceFromContext(ctx)

	resp := &logsv1.PingResponse{
		Message: message,
		TraceId: traceID,
		SpanId:  spanID,
	}
	return connect.NewResponse(resp), nil
}
