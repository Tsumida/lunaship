# Goal

- Feature: add new http endpoint `/pprof/start` and `/pprof/stop` to start and stop pprof server dynamically. This allows users to enable pprof when needed without restarting the server.

# Facts & requirements
- The pprof server should be started on a separate port (default 6060) to avoid conflicts with the main application server.

# Design

## Lifecycle

pprof state machine:
- `/pprof/stop`: `Running` -> `Dumping` -> `Stopped`, stop pprof and dump profiles. If dumping failed, print an error log (metric is needed). 
- `/pprof/start`: `Stopped` -> `Running`, start pprof. 
- `/pprof/stop`: `Stopped` -> `Stopped`, do nothing.
- `/pprof/start`: `Running` -> `Running`, do nothing. 

We expect no concurrent profiling. So use state & atomic to control the lifecycle of pprof.

## Profile dumping

When stopping pprof, we should dump the profiles to files for later analysis. The profiles to dump include:
- CPU profile
- Heap profile
- Goroutine profile

File names should be in `/tmp/pprof-<APP_NAME>-<YYYYMMDD-hhmmss>-<PROFILE>.prof` format for easy identification.
`<PROFILE>` is one of: `cpu`, `heap`, `goroutine`.

## How to use

### Key points

- `/pprof/start` and `/pprof/stop` are exposed on the main HTTP server (same port as the business endpoints, e.g. `8080`).
- The actual pprof server (`/debug/pprof/*`) is started dynamically on a separate port (default `6060`) after `/pprof/start`.
- Ensure `APP_NAME` is set in the process environment; it is used in the dumped profile filename.

### Kubernetes workflow (recommended)

1. Port-forward main HTTP port and start profiling:

```bash
POD=logs-demo-b-xxxx
NS=test

kubectl -n "$NS" port-forward "pod/$POD" 8080:8080

BASE_URL=http://127.0.0.1:8080 bash scripts/pprof.sh start
```

2. Run your workload, then stop profiling:

```bash
BASE_URL=http://127.0.0.1:8080 bash scripts/pprof.sh stop
```

The stop response includes `dumped_files` (full paths under `/tmp`).

3. Copy the profile out of the container:

```bash
kubectl -n "$NS" cp "$POD:/tmp/pprof-<APP_NAME>-<YYYYMMDD-hhmmss>-cpu.prof" ./tmp/
kubectl -n "$NS" cp "$POD:/tmp/pprof-<APP_NAME>-<YYYYMMDD-hhmmss>-heap.prof" ./tmp/
kubectl -n "$NS" cp "$POD:/tmp/pprof-<APP_NAME>-<YYYYMMDD-hhmmss>-goroutine.prof" ./tmp/
```

### Optional: browse `/debug/pprof/*` after start

After profiling is started, you can port-forward the pprof port:

```bash
kubectl -n "$NS" port-forward "pod/$POD" 6060:6060
```

Then access:

- `http://127.0.0.1:6060/debug/pprof/`
- `http://127.0.0.1:6060/debug/pprof/goroutine?debug=2`
