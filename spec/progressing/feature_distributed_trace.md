# Goal
- New feature: verification of distributed trace reporting with Jaeger as backend.

# Fact

- We have implemented a distributed trace reporter in `infra/trace.go` and a trace interceptor in `interceptor/trace.go`.
- The reporter starts at `RunAfterInit()` right after `InitLog()`.
- For each request, `NewTraceInterceptor()` extracts the trace context from the request header and creates a new span for the request. The span is ended when the request is finished. (see `example/logs/main.go` and `interceptor/trace.go`)

# Plan

## Deployment

- Jaeger backend and Lunaship demo services run in the `test` namespace.
- Jaeger backend: deployed on remote Docker. We expose it to the k3s cluster via Traefik rules and a Service/Endpoints pair.
- Two Lunaship services running in k3s. We send requests and they report traces to Jaeger.
- Tinyinfra: remote host owns Docker and k3s.

### Jaeger Connectivity
- The services must be configured to reach Jaeger. Use `JAEGER_COLLECTOR_ENDPOINT` for remote access.
- We route Jaeger collector traffic through Traefik with `example/logs/deploy/jaeger-traefik.yaml`.
- Set `JAEGER_COLLECTOR_ENDPOINT` to `http://traefik.kube-system.svc.cluster.local/jaeger/api/traces` in the deployments.
- Ensure Jaeger collector HTTP port `14268` is reachable on the remote Docker host (see `example/logs/docker-compose.yaml`).
- Use const sampling to report all traces: set `JAEGER_SAMPLER_TYPE=const` and `JAEGER_SAMPLER_RATE=1`.

## Todo
You should make sure that the trace reporter works correctly and can report traces to Jaeger.
The key part is: running 2 services (defined at `example/logs/main.go`) and send requests, so we can check the result in jaeger GUI. 

So we should do the following steps:
1. Build the image for testing (Dockerfile is a scaffold and may be updated). Example:
```sh
docker build -t lunaship-logs-demo:latest -f example/logs/Dockerfile .
```
We also need to push the image to the k3s cluster by uploading the image file and then load it by `ctr`.
2. Apply the Traefik rules and demo deployments:
   - Update `example/logs/deploy/jaeger-traefik.yaml` with the tinyinfra host IP.
   - Apply `example/logs/deploy/namespace.yaml`, `example/logs/deploy/jaeger-traefik.yaml`, and `example/logs/deploy/logs-demo.yaml`.
3. We expect two services so one can call another inside the `test` namespace by:
```sh
curl -sS -X POST \
	-H "Content-Type: application/json" \
	-H "Connect-Protocol-Version: 1" \
	--data '{"target":"logs-demo-b:8080","endpoint":"/logs.v1.DummyService/Ping","request_json":"{}"}' \
	http://logs-demo-a:8080/logs.v1.DummyService/Transfer
```
4. The user runs `ssh -L 16686:localhost:16686 tinyinfra` so we can verify the results in the Jaeger UI.

# Scope

## Source code
- Distributed Trace reporter and interceptor defined at: `infra/trace.go` and `interceptor/trace.go`. 
- Examples defined at `example/logs/*`. 
- Dockerfile scaffold defined at `example/logs/Dockerfile`.
- Traefik and deployment manifests defined at `example/logs/deploy/*`.

## Out-of-scope

- Anything not related to distributed trace reporter and interceptor. Keep changes under the Source code part.

# Verification & Test cases

## Normal
service A calls service B, and we can see the trace in Jaeger.
We can gather the trace and span by traceID or spanID. 

Correctness: 
- Verify service spans only (server spans). Ignore client spans if they exist.
- Expect at least two service spans: one for A and another for B.
- Span of B (the callee) has a parent span ID pointing to the span of A (the caller).

## Error handling
A calls B with a wrong service-name, so there may be connection error. 

Correctness: 
- Verify service spans only (server spans). Ignore client spans if they exist.
- Expect no service span for B.
- The service span of A should have error tag, and the error message should indicate the connection error.

# Verification Log

## Steps (Latest)
1. `make build`
2. (Optional) `make upload` and manually import image on tinyinfra.
3. `make demo-logs` or `make demo-logs IMPORT=1` if you want auto import.
4. Verify Jaeger UI via `ssh -L 16686:localhost:16686 tinyinfra`.

## Problems & Status
- [solved] Docker build failed at `go mod tidy` inside `example/logs/Dockerfile`. Fixed by removing `go mod tidy` from the image build step.
- [solved] Demo is not replayable due to import/`sudo` flow. `make demo-logs` now skips import unless `IMPORT=1`.
- [solved] `scp` target mismatch. Updated default target to `tsumida@tinyinfra`.
- [solved] `curlimages/curl` pull blocked. Switched demo client to use `busybox` + `wget`.
- [solved] Service routing caused intermittent `No route to host` when using service DNS from the curl pod. Switched demo client to hit pod IPs directly.
- [solved] Const sampling required. Set `JAEGER_SAMPLER_TYPE=const` and `JAEGER_SAMPLER_RATE=1` and enforced const behavior in config.
- [solved] Need trace config visibility. Added startup log for trace configuration and reporter error logging + metric.
- [todo] Jaeger UI still shows no traces after endpoint update to `192.168.0.120`. Verify Traefik route to `/jaeger/api/traces`, collector port `14268` reachability, and Jaeger ingestion logs.
- [todo] If reporter errors appear, confirm `lunaship_trace_reporter_errors_total` increments via `/metrics` (requires `PROMETHEUS_LISTEN_ADDR` to be set and `infra.InitMetric` running).
- [ignored] Endpoints deprecation warning in Kubernetes v1.33+ (informational).

# Extensions 

No 
