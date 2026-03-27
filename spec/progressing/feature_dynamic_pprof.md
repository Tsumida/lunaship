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

File names should be in /tmp/pprof-<APP_NAME>-<TIMESTAMP>-<END_TIMESTAMP>.prof format for easy identification.