package main

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	logsv1 "github.com/tsumida/lunaship/example/logs/gen"
	"github.com/tsumida/lunaship/example/logs/gen/logsv1connect"
	"github.com/tsumida/lunaship/interceptor"
	"github.com/tsumida/lunaship/log"
	"google.golang.org/protobuf/encoding/protojson"
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

func (s *DummyService) Transfer(
	ctx context.Context,
	req *connect.Request[logsv1.TransferRequest],
) (*connect.Response[logsv1.TransferResponse], error) {
	targetAddr := strings.TrimSpace(req.Msg.GetTargetAddr())
	if targetAddr == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("target_addr is required"))
	}
	endpoint := strings.TrimSpace(req.Msg.GetEndpoint())
	if endpoint == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("endpoint is required"))
	}
	if !strings.HasPrefix(endpoint, "/") {
		endpoint = "/" + endpoint
	}
	if endpoint != logsv1connect.DummyServicePingProcedure {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("unsupported endpoint"))
	}

	requestJSON := strings.TrimSpace(req.Msg.GetRequestJson())
	if requestJSON == "" {
		requestJSON = "{}"
	}
	pingReq := &logsv1.PingRequest{}
	if err := protojson.Unmarshal([]byte(requestJSON), pingReq); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	client := logsv1connect.NewDummyServiceClient(
		http.DefaultClient,
		baseURL(targetAddr),
		connect.WithInterceptors(interceptor.NewTraceClientInterceptor()),
	)
	resp, err := client.Ping(ctx, connect.NewRequest(pingReq))
	if err != nil {
		return nil, err
	}
	respJSON, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(resp.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&logsv1.TransferResponse{
		ResponseJson: string(respJSON),
	}), nil
}

func baseURL(targetAddr string) string {
	targetAddr = strings.TrimSpace(targetAddr)
	if strings.HasPrefix(targetAddr, "http://") || strings.HasPrefix(targetAddr, "https://") {
		return strings.TrimRight(targetAddr, "/")
	}
	return "http://" + strings.TrimRight(targetAddr, "/")
}
