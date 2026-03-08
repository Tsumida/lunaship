# Goal

- Add stable RPC server metrics for request count, latency, and error distribution.
- Keep implementation maintainable by separating concerns: tracing and metrics should be independent interceptors.

# Facts

- Current server tracing interceptor: `interceptor/trace.go` (`NewTraceInterceptor`).
- Existing prometheus endpoint setup: `infra/metric.go` (`InitMetric`).
- Service bootstrap path: `service/setup.go` (`RunAfterInit`).
- Existing log/error code pattern already uses `connect.CodeOf(err)` in interceptor code.

# Scope

- In scope:
  - RPC server-side metrics only.
  - Unary requests handled by connect-go server interceptors.
  - Prometheus exposition via `/metrics`.
- Out of scope:
  - Client-side RPC metrics.
  - Streaming-specific metrics.
  - Grafana dashboard JSON/templates in this PRD.

# Design

## Interceptor architecture

- Add a dedicated metrics interceptor, do not merge into `NewTraceInterceptor`.
- Suggested API:
  - `interceptor.NewMetricsInterceptor() connect.UnaryInterceptorFunc`
- Reason:
  - Better separation of concerns.
  - Easier unit testing and future rollout toggles.

## Metric definitions

- Counter:
  - Name: `rpc_server_requests_total`
  - Type: `CounterVec`
  - Labels:
    - `service`: from `SERVICE_ID` env (same naming convention as `service/setup.go`).
    - `endpoint`: from `req.Spec().Procedure`, e.g. `/acme.foo.v1.FooService/Bar`.
    - `code`: from `connect.CodeOf(err).String()`; for success use `OK`.
- Histogram:
  - Name: `rpc_server_request_duration_seconds`
  - Type: `HistogramVec`
  - Labels:
    - `service`
    - `endpoint`
    - `code`
  - Value: request duration in seconds.
  - Buckets:
    - Start with Prometheus default buckets, and tune after observing p95/p99 in staging.

## Error/code semantics

- Use connect status code, not HTTP response header parsing.
- Mapping:
  - `err == nil` -> `OK`
  - `err != nil` -> `connect.CodeOf(err).String()`
- This keeps semantics aligned with connect-go and existing project patterns.

## Label and cardinality strategy

- Keep label set minimal (`service`, `endpoint`, `code`).
- Do not add `method`; connect-go unary server currently uses POST and this label has near-zero analytical value.
- Cardinality estimation (upper bound across ecosystem):
  - Endpoints: 100
  - Services: 200
  - Codes: ~17 connect codes
  - Total per metric upper bound: 340,000
- Practical note:
  - Per-process cardinality is much lower because one service process emits one `service` value.

## Metrics reporter startup

- `infra.InitMetric` is blocking (`ListenAndServe`), so calling it directly in synchronous init path can block server startup.
- Requirement:
  - Start metrics endpoint in a non-blocking goroutine, or refactor `InitMetric` to support non-blocking startup lifecycle.
  - Avoid startup deadlock between business server and metrics server.

# Plan

1. Implement `NewMetricsInterceptor` in `interceptor/metrics.go`.
2. Register two Prometheus collectors (`CounterVec`, `HistogramVec`) with `prometheus.MustRegister`.
3. Wire interceptor in service handler setup (same place where trace/logger interceptors are composed).
4. Ensure metrics endpoint starts without blocking service startup.
5. Add tests:
   - Unit tests for interceptor metric updates.
   - Integration test for `/metrics` exposition.

# Verification

## Automated tests

- Unit:
  - Assert counter increments for success request (`code="OK"`).
  - Assert counter increments for error request (`code=<non-OK>`).
  - Assert histogram receives observations for both success/error.
  - Assert labels include expected `service` and `endpoint`.
- Integration:
  - Start test server with metrics interceptor + metrics endpoint.
  - Send at least one success and one error request.
  - Scrape `/metrics` and assert metric families/label sets are present.

## Manual verification

- Run service with metrics enabled.
- Trigger representative RPC traffic.
- Check `/metrics` contains:
  - `rpc_server_requests_total`
  - `rpc_server_request_duration_seconds_bucket`
  - `rpc_server_request_duration_seconds_count`
  - `rpc_server_request_duration_seconds_sum`
- Build dashboard panels for:
  - QPS by endpoint/code.
  - Error ratio by endpoint.
  - p95/p99 latency by endpoint.

# Acceptance criteria

- Metrics are exported at `/metrics` without blocking service startup.
- Every unary request produces:
  - one counter increment
  - one histogram observation
- Success and error requests are distinguishable by `code` label.
- Labels are stable and low-noise: `service`, `endpoint`, `code`.
- Tests cover both success and error flows and pass in CI.

# Risks and mitigations

- Risk: duplicate collector registration in tests.
  - Mitigation: isolate registries in tests or guard global registration.
- Risk: endpoint label cardinality spikes if procedure names become dynamic.
  - Mitigation: enforce static procedure path usage; reject dynamic path templates.
- Risk: metrics endpoint port conflicts.
  - Mitigation: allow configurable `PROMETHEUS_LISTEN_ADDR` per environment/test.
