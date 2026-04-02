# Goal

- Add `app.toml` for application initialization.
- Put all Redis, MySQL, Kafka config into one place. 
- Allow custom config with auto serialization and validation.

# Background

We use environment variables for configuration and there are some issues:
- Env definition are scattered in different places, hard to maintain.
- Ugly deserialization when passing a structured config through env vars.

So we want a module `config/` to init load the `config/app.toml` and provide APIs for other modules to get the config.

# Out of scope

- `app.toml` stores static config that will not change at runtime(If there are any changes, just restart).
- There are information need to be injected by env vars at startup, which is listed at section `Environment variables`.
- Dynamic listening and reloading. 

# Design 

## Initialization

We assume that `config/app.toml` is already mounted to the application container before startup (like configmap in k8s, or bind mount in docker).

The `config/app.toml` is loaded and fixed at startup. We don't support dynamic reloading for now.

If the config is invalid, such as lack of `APP_NAME`, the application will print the error log and then panic. 

Auto-initialization for Redis, MySQL and Kafka clients at startup. For example, we can init multiple clients for multiple MySQL endpoints, and access a specific client by name, e.g. `GetMySQLClient("dev-mysql")`.


## Config struct

Key parts:
- Service metadata like `APP_NAME`, `COMMIT_HASH`, `LOG_LEVEL`
- DB & middleware endpoints with multiple instances support (named instances).
- Other custom config. 

## Config example:

```toml
[app]

# Required. The unique name of the application.
app_name = "logs-demo"

[app.log]
# Optional. Default is "info". debug, info, warn, error
level = "info" 

[app.trace]
enabled = true # optional, default is false. Enable OpenTelemetry tracing.
otel_exporter_otlp_endpoint = "otel.tinyinfra.dev:4317" 
otel_exporter_otlp_protocol = "http" # optional, with http supported only. 
otel_resource_otlp_trace_endpoint = "http://192.168.0.120:4318"

[app.pprof]
# Optional. Enable pprof endpoint for profiling. Allow "server" and "dynamic". 
# "server" means start a separate pprof server that keeps running. 
# "dynamic" means start a pprof server on demand when receiving a signal (e.g. /pprof/start, /pprof/stop). The pprof server will stop after a certain duration (e.g. 5 minutes) to avoid potential security risk.
mode = "dynamic" 
enabled = true # optional, default is true. Enable pprof endpoint.


# ======================================================= global config for all Redis instances
[redis] 
[redis.dev-redis] # An endpoint named "dev-redis"
addr = "redis.tinyinfra.dev"
port = 6379

# ======================================================= global config for all MySQL instances
[mysql] 
max_open_conns = 300
max_idle_conns = 100
max_idle_time = "30s"

[mysql.dev-mysql] # An endpoint named "dev-mysql"
host = "mysql.tinyinfra.dev"
port = 3306
username = "root"
password = "password" # demo only. In production, source it from a k8s Secret and render it into the file at deploy time.
database = "test" # required.  

# optional, default is false. Ping the MySQL server at startup to validate the connection. 
# Panic if ping failed. 
ping_enabled = true

# ======================================================= global config for all Kafka instances
[kafka] 
[kafka.producer.dev] # An producer endpoint named "producer.dev"
brokers = ["kafka.tinyinfra.dev:9092"]
topic = "test-topic"
acks = "all" # default is "all". Other options: "0", "1".

[kafka.consumer.dev] # An consumer endpoint named "consumer.dev"
brokers = ["kafka.tinyinfra.dev:9092"]
topic = "test-topic"
consumer_group = "demo-consumer-group" # required for consumer group based consumption.

# ======================================================= custom
# custom config for a module named "custom-kv". There may be multiple custom config sections. 
[custom-kv] 
key1 = "value1"
key2 = "value2"
blocked_list = ["user1", "user2"] # optional, default is empty list.
```

## Deserialization & validation

We can define a struct for custom configuration. For example:

```go
type ModuleKeyValue struct {
    Key1 string `toml:"key1"`
    Key2 string `toml:"key2"`
    BlockedList []string `toml:"blocked_list"`
}

func FromAppToml(body []byte) (*ModuleKeyValue, error) {
    var config ModuleKeyValue
    if err := UnmarshalToml(body, ".custom-kv", &config); err != nil {
        log.Error("failed to unmarshal app.toml", "error", err)
        return nil, err
    }
    return &config, nil
}
```
# Environment variables

There are part of environment variables that are still needed for runtime overrides / injected runtime metadata:
```sh
ENV ENV=""
ENV ERR_FILE=""
ENV LOG_FILE=""
ENV COMMIT_HASH=""
```

There should be injected by CICD pipeline.

Redis / MySQL / Kafka endpoints (host, port, user, password, etc.) should live in `app.toml` rather than env vars (migration goal).

And some var should keep unchanged: 

```sh
JWT_MAILBOX_KEY
```

# Migration

## MySQL
- Add `GetMySQLClient(name string) (*gorm.DB, error)` to get the MySQL client by name.
- `GlobalMySQL` is the same as `GetMySQLClient("default")` for backward compatibility. We can deprecate `GlobalMySQL` in the future.

## Redis
- Add `GetRedisClient(name string) (redis.UniversalClient, error)` to get the Redis client by name.
- `GlobalRedis` is the same as `GetRedisClient("default")`.

# Suggestions

for future improvement:
- Error handling: instead of panicking inside the config layer, prefer returning an error to the service bootstrap (so `main` / `svc.RunAfterInit()` decides to exit, retry, or degrade). Keep the current "panic on invalid config" behavior for v1 if needed, but design APIs to allow future change without breaking callers.
- Custom config: the current `UnmarshalToml(body, ".custom-kv", &out)` approach is good enough for v1. A better next step is: parse `app.toml` once into an in-memory representation and provide a helper like `DecodeSection(section string, out any) error` with caching, strict/lenient unknown-field modes, and shared validation.
- Secrets: `password` in `app.toml` is acceptable for demo. For real deployments, store secrets in a k8s Secret and render them into `app.toml` at deploy time (or support `*_file` / `*_env` indirection later).

# Deployment

We need a configmap that contains the `app.toml` file for each application. The configmap can be generated from a template file with environment variable substitution.

## Example: app_config.yaml
todo 


# Todo list

- Phase 1: create `config/` package with a clear entrypoint such as `Load(path string) (*AppConfig, error)` and `LoadFromBytes(body []byte) (*AppConfig, error)`.
- Phase 1: define the root config structs for `app`, `app.log`, `app.trace`, `app.pprof`, `redis`, `mysql`, and `kafka`.
- Phase 1: decide and document the v1 defaulting rules inside the loader.
- Phase 1: implement TOML parsing with strict validation for required fields like `app.app_name`.
- Phase 1: return structured errors from loading / validation instead of hiding parse or validation details.

- Phase 2: add custom-section decoding support, for example `DecodeSection[T any](cfg *AppConfig, section string) (*T, error)`.
- Phase 2: keep the v1 implementation simple: parse once, retain the raw section tree, and decode sub-sections from memory.
- Phase 2: decide whether unknown fields are allowed for custom sections in v1 and enforce that consistently.
- Phase 2: add validation helpers for common checks such as non-empty strings, valid ports, and non-empty broker / topic lists.

- Phase 3: add tests for successful load from a complete `app.toml`.
- Phase 3: add tests for malformed TOML.
- Phase 3: add tests for missing required fields.
- Phase 3: add tests for named instances like `mysql.dev-mysql` and `redis.dev-redis`.
- Phase 3: add tests for custom section decoding and default values.

- Phase 4: add a small example config file under `example/` or `spec/` for local development.
- Phase 4: define how the app finds the file path in v1, for example a fixed `config/app.toml` path or a single override env like `APP_CONFIG_PATH`.
- Phase 4: expose read-only accessors for loaded config so later integration does not depend on raw TOML bytes.

- Phase 5: prepare migration hooks without replacing current env usage yet.
- Phase 5: add placeholder APIs for future named client lookup such as `GetMySQLClient(name)` and `GetRedisClient(name)`, but keep them unused until config loading is stable.
- Phase 5: document which env vars stay as runtime metadata and which fields are expected to move into `app.toml`.

- Verification: `go test ./...` should pass after the config package and tests are added.
- Verification: add a focused package test target for config loading so iteration stays fast.
- Verification: verify one manual happy path by loading an example `app.toml` from a tiny demo or test helper.
