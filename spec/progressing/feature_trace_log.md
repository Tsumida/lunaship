# Goal

- New feature: Better log keys for tracing and debugging, including server and trace information.
- New feature: Aligned with observability infrastructure (k3s, loki, prometheus, jaeger, grafana). 

# Facts
- The service is running in k3s cluster, and we have loki for log collection, jaeger for tracing, and prometheus for metrics. Grafana is used for visualization.
- Code of logging is under `interceptor/req_rsp_logger.go`, and we can modify it to include server and trace information.

# Plan: 

## Logging with server and trace information. 

While we use `infra.GlobalLog()` or any log derived from it, we want to automatically include server and trace information in the log. This will help us to trace the request and gather all logs together for reasoning.

All fields start with `_` to avoid conflict with existing log fields.
All required fields must appear in the final output; the logging system may auto-inject them if the caller does not provide them.

## Required rules
- `Required`: Must present in the final log output. Zero-value are injected if not provided by caller. The logging system must ensure these fields are always present in the final log output.
- `Optional`: May be absent in the final log outpu if not provided by caller.

### Common Fields
All internal logging should include the following fields:
- `_level`: The log level, such as `debug`, `info`, `warn`, `error`. Required. Type=string
- `_env`: "prod", "test". Required. Type=string
- `_app`: The name of the Application, comes from env "APP_NAME". Required. Type=string
- `_app_ip`: The IP address of the application. Required. Type=string
- `_app_port`: The port of the application. Required. Type=int
- `_msg`: The log message. Required. Truncated if there are more than 1024 UTF-8 characters. Type=string
- `_trace_id`: The trace ID for the request. Required. Type=string
- `_span_id`: The span ID for the log entry. Required. Type=string
- `_parent_span_id`: The parent span ID for the log entry. Optional, can be empty if there is no parent span. Type=string
- `_ts`: UTC timestamp of the log entry in milliseconds. Required, if not provided, the logging system will automatically add it. Type=int

Truncation rules:
- Truncate `_msg` and `_sql` to 1024 UTF-8 characters (character count, not bytes) before JSON encoding.


Usage example:
```go
func main(){
    log := infra.GlobalLog()
    log.Info("Application submitted")
}
```

Example log output:
```json
// ok
{
    "_level": "info",
    "_app_ip": "192.168.0.100",
    "_app_port": 8080,
    "_app": "user-service",
    "_env": "prod",
    "_msg": "Application submitted",
    "_trace_id": "1a2b3c4d5e6f7g8h9i0j",
    "_span_id": "1234567890abcdef",
    "_parent_span_id": "",
    "_ts": 1771521635000
}
```
err:
```json
{
    "_level": "error",
    "_app_ip": "192.168.0.100",
    "_app_port": 8080,
    "_app": "user-service",
    "_env": "prod",
    "_msg": "failed to connect to database",
    "_trace_id": "1a2b3c4d5e6f7g8h9i0j",    
    "_span_id": "1234567890abcdef",
    "_parent_span_id": "",
    "_ts": 1771521635000
}
```

### RPC logging
While service A(@192.168.0.101:8080) calls service B(@192.168.0.102:8081), both of them print respective logs.
RPC logs should include the following additional fields:
- `_dur_ms`: The duration of the RPC call in milliseconds. Optional, can be added by logging system if not provided. Type=int
- `_caller_ip`: The IP address of the caller. Required. Type=string
- `_caller_port`: The port number of the caller. Required. Type=int
- `_caller_app`: The name of the caller service. Required. Type=string
- `_callee_ip`: The IP address of the callee. Required. Type=string
- `_callee_port`: The port number of the callee. Required. Type=int
- `_callee_app`: The name of the callee service. Required. Type=string

For service A,  it prints:
```json
{
    "_level": "info",
    "_app_ip": "192.168.0.101",
    "_app_port": 8080,
    "_app": "A",
    "_env": "prod",
    "_caller_ip": "192.168.0.101",
    "_caller_port": 8080,
    "_caller_app": "A",
    "_callee_ip": "192.168.0.102",
    "_callee_port": 8081,
    "_callee_app": "B",
    "_msg": "RPC",
    "_trace_id": "1a2b3c4d5e6f7g8h9i0j",
    "_span_id": "1234567890abcdef",
    "_parent_span_id": "",
    "_ts": 1771521635000,
}
```
For service B: 
```json
{
    "_level": "info",
    "_app_ip": "192.168.0.102",
    "_app_port": 8081,
    "_app": "B",
    "_env": "prod",
    "_caller_ip": "192.168.0.101",
    "_caller_port": 8080,
    "_caller_app": "A",
    "_callee_ip": "192.168.0.102",
    "_callee_port": 8081,
    "_callee_app": "B",
    "_msg": "Called by service A",
    "_trace_id": "1a2b3c4d5e6f7g8h9i0j",
    "_span_id": "1234567890abcdef",
    "_parent_span_id": "",
    "_ts": 1771521635000,
}
```

### Redis logging
The caller reports the trace (since we can't modify Redis): 
- `_dur_ms`: The duration of the redis call in milliseconds. Optional, can be added by logging system if not provided. Type=int
- `_instance_ip`: The IP address of the Redis server. Required. Type=string
- `_instance_port`: The port number of the Redis server. Required. Type=int
- `_redis_lua_sha`: The SHA1 of the Redis Lua script. Optional. Type=string
  
Example log:
```json
{
    "_level": "info",
    "_app_ip": "192.168.0.102",
    "_app_port": 8081,
    "_app": "B",
    "_env": "prod",
    "_instance_ip": "192.168.0.103",
    "_instance_port": 6379,
    "_redis_lua_sha": "abcdef1234567890",
    "_msg": "Executing Redis command",
    "_trace_id": "1a2b3c4d5e6f7g8h9i0j",
    "_span_id": "1234567890abcdef",
    "_parent_span_id": "",
    "_ts": 1771521635000
}
```

### MySQL logging
- `_instance_ip`: The IP address of the MySQL server. Required. Type=string
- `_instance_port`: The port number of the MySQL server. Required. Type=int
- `_database`: The name of the database. Required. Type=string
- `_dur_ms`: The duration of the SQL execution in milliseconds. Optional, can be added by logging system if not provided. Type=int
- `_sql`: The final SQL statement being executed, with parameters. Optional. Truncated if there are more than 1024 UTF-8 characters.
- `_is_slow_query`: The flag indicating whether the SQL is a slow query. Optional, can be added by logging system if not provided.

Example log:
```json
{
    "_level": "info",
    "_app_ip": "192.168.0.102",
    "_app_port": 8081,
    "_app": "B",
    "_instance_ip": "192.168.0.104",
    "_instance_port": 3306,
    "_database": "example_db",
    "_sql": "SELECT * FROM users WHERE id = 1",
    "_dur_ms": 120,
    "_env": "prod",
    "_is_slow_query": false,
    "_msg": "SQL",
    "_trace_id": "1a2b3c4d5e6f7g8h9i0j",
    "_span_id": "1234567890abcdef",
    "_parent_span_id": "",
    "_ts": 1771521635000
}
```

# Verification
1. User executes `make build, make upload` and then import images into k3s (at `192.168.0.120`). 
2. User the executes `make demo-logs` to apply or rollout the pods. 
3. When pod is ready, we can use `kubectl logs -n test  <pod-name>` to check the logs. We should see the above fields in the log output.
4. User opens Grafana, queries {_trace_id="xxxx"} in Loki, and clicks the 'Jaeger' button to see the full trace