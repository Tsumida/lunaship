package main

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	redis "github.com/redis/go-redis/v9"
	logsv1 "github.com/tsumida/lunaship/example/logs/gen"
	"github.com/tsumida/lunaship/example/logs/gen/logsv1connect"
	"github.com/tsumida/lunaship/interceptor"
	"github.com/tsumida/lunaship/log"
	"github.com/tsumida/lunaship/mysql"
	lunaredis "github.com/tsumida/lunaship/redis"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	redisDemoCounterKey = "logs_demo:get_spot:counter"
	redisDemoLastReqKey = "logs_demo:get_spot:last_request"
	redisDemoLuaScript  = `return redis.call("INCR", KEYS[1])`
)

type DummyService struct {
	luaOnce sync.Once
	luaErr  error
	luaExec *lunaredis.LuaExecutor
}

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

func (s *DummyService) GetSpot(
	ctx context.Context,
	req *connect.Request[logsv1.GetSpotRequest],
) (*connect.Response[logsv1.GetSpotResponse], error) {
	if err := s.exerciseRedis(ctx); err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}

	db := mysql.GlobalMySQL()
	if db == nil {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("mysql is not initialized"))
	}

	limit := req.Msg.GetLimit()
	if limit == 0 {
		limit = 10
	}
	if limit > 200 {
		limit = 200
	}
	offset := req.Msg.GetOffset()

	var rows []SpotModel
	if err := db.WithContext(ctx).
		Order("id ASC").
		Limit(int(limit)).
		Offset(int(offset)).
		Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	spots := make([]*logsv1.Spot, 0, len(rows))
	for _, row := range rows {
		spots = append(spots, toProtoSpot(row))
	}

	return connect.NewResponse(&logsv1.GetSpotResponse{
		Spots: spots,
	}), nil
}

func (s *DummyService) exerciseRedis(ctx context.Context) error {
	client := lunaredis.GlobalRedis()
	if client == nil {
		return errors.New("redis is not initialized")
	}
	if err := client.Set(ctx, redisDemoLastReqKey, "GetSpot", 30*time.Second).Err(); err != nil {
		return err
	}
	if _, err := client.Get(ctx, redisDemoLastReqKey).Result(); err != nil {
		return err
	}
	if err := s.prepareRedisLua(ctx, client); err != nil {
		return err
	}
	return s.luaExec.UpdateOneEvent(ctx, []string{redisDemoCounterKey})
}

func (s *DummyService) prepareRedisLua(ctx context.Context, client redis.UniversalClient) error {
	s.luaOnce.Do(func() {
		s.luaExec = lunaredis.NewLuaExecutor(
			"logs_demo_get_spot",
			client,
			redisDemoLuaScript,
			nil,
		)
		s.luaErr = s.luaExec.PrepareLuaScript(ctx)
	})
	return s.luaErr
}

func baseURL(targetAddr string) string {
	targetAddr = strings.TrimSpace(targetAddr)
	if strings.HasPrefix(targetAddr, "http://") || strings.HasPrefix(targetAddr, "https://") {
		return strings.TrimRight(targetAddr, "/")
	}
	return "http://" + strings.TrimRight(targetAddr, "/")
}
