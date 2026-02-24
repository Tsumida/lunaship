# PRD
## Target

- New feature: Logging with server and trace information. 

# Design 

## Logging with server and trace information. 

While we use `infra.GlobalLog()` or any log derived from it, we want to automatically include server and trace information in the log. This will help us to trace the request and gather all logs together for reasoning.

All fields start with `_` to avoid conflict with existing log fields.
All required fields must appear in the final output; the logging system may auto-inject them if the caller does not provide them.

### Common Fields
All internal logging should include the following fields:
- `_level`: The log level, such as `debug`, `info`, `warn`, `error`. Required. Type=string
- `_env`: "prod", "test". Required. Type=string
- `_server_ip`: The IP address of the server. Required. Type=string
- `_server_port`: The port of the server. Required. Type=int
- `_service`: The name of the caller service. Required. Type=string
- `_msg`: The log message. Required. Truncated if there are more than 1024 UTF-8 characters. Type=string
- `_trace_id`: The trace ID for the request. Required. Type=string
- `_span_id`: The span ID for the log entry. Required. Type=string
- `_parent_span_id`: The parent span ID for the log entry. Optional, can be empty if there is no parent span. Type=string
- `_err`: The error flag, true if there is an error. Optional, default is false. Set true when level is `error` or an error is attached. Type=boolean
- `_ts`: UTC timestamp of the log entry in milliseconds. Required, if not provided, the logging system will automatically add it. Type=int
- `_ts_us`: UTC timestamp of the log entry in microseconds. Optional, if not provided, the logging system will automatically add it. Type=int

Truncation rules:
- Truncate `_msg` and `_sql` to 1024 UTF-8 characters (character count, not bytes) before JSON encoding.

Server identity rules:
- `_server_ip` and `_server_port` should represent the listening address that accepted the request. If there is no request context (background job), use the configured primary service address.

Trace context availability:
- If the trace context is missing, the logging system must generate a new `_trace_id` and `_span_id`, and set `_parent_span_id` to empty.

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
    "_server_ip": "192.168.0.100",
    "_server_port": 8080,
    "_service": "user-service",
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
    "_server_ip": "192.168.0.100",
    "_server_port": 8080,
    "_service": "user-service",
    "_env": "prod",
    "_msg": "failed to connect to database",
    "_trace_id": "1a2b3c4d5e6f7g8h9i0j",    
    "_span_id": "1234567890abcdef",
    "_parent_span_id": "",
    "_err": true,
    "_ts": 1771521635000
}
```

### RPC
While service A(@192.168.0.101:8080) calls service B(@192.168.0.102:8081), both of them print respective logs.
RPC logs should include the following additional fields:
- `_duration_ms`: The duration of the RPC call in milliseconds. Optional, can be added by logging system if not provided. Type=int
- `_caller_ip`: The IP address of the caller. Required if server is callee. Type=string
- `_caller_port`: The port number of the caller. Required if server is callee. Type=int
- `_caller_service`: The name of the caller service. Required if server is callee. Type=string
- `_callee_ip`: The IP address of the callee. Required if server is caller. Type=string
- `_callee_port`: The port number of the callee. Required if server is caller. Type=int
- `_callee_service`: The name of the callee service. Required if server is caller. Type=string
- `_rpc_type`: `caller` or `callee`. Required.

For service A,  it prints:
```json
{
    "_level": "info",
    "_server_ip": "192.168.0.101",
    "_server_port": 8080,
    "_service": "A",
    "_env": "prod",

    "_caller_ip": "192.168.0.101",
    "_caller_port": 8080,
    "_caller_service": "A",
    "_callee_ip": "192.168.0.102",
    "_callee_port": 8081,
    "_callee_service": "B",
    "_msg": "RPC",
    "_trace_id": "1a2b3c4d5e6f7g8h9i0j",
    "_span_id": "1234567890abcdef",
    "_parent_span_id": "",
    "_ts": 1771521635000,
    "_rpc_type": "caller"
}
```
For service B: 
```json
{
    "_level": "info",
    "_server_ip": "192.168.0.102",
    "_server_port": 8081,
    "_service": "B",
    "_env": "prod",
    "_caller_ip": "192.168.0.101",
    "_caller_port": 8080,
    "_caller_service": "A",
    "_callee_ip": "192.168.0.102",
    "_callee_port": 8081,
    "_callee_service": "B",
    "_msg": "Called by service A",
    "_trace_id": "1a2b3c4d5e6f7g8h9i0j",
    "_span_id": "1234567890abcdef",
    "_parent_span_id": "",
    "_ts": 1771521635000,
    "_rpc_type": "callee"
}
```

### Redis
The caller reports the trace (since we can't modify Redis): 
- `_duration_ms`: The duration of the redis call in milliseconds. Optional, can be added by logging system if not provided. Type=int
- `_instance_ip`: The IP address of the Redis server. Required. Type=string
- `_instance_port`: The port number of the Redis server. Required. Type=int
- `_redis_lua_sha1`: The SHA1 of the Redis Lua script. Optional. Type=string
  
Example log:
```json
{
    "_level": "info",
    "_server_ip": "192.168.0.102",
    "_server_port": 8081,
    "_service": "B",
    "_env": "prod",
    "_instance_ip": "192.168.0.103",
    "_instance_port": 6379,
    "_redis_lua_sha1": "abcdef1234567890",
    "_msg": "Executing Redis command",
    "_trace_id": "1a2b3c4d5e6f7g8h9i0j",
    "_span_id": "1234567890abcdef",
    "_parent_span_id": "",
    "_ts": 1771521635000
}
```

### MySQL
- `_instance_ip`: The IP address of the MySQL server. Required. Type=string
- `_instance_port`: The port number of the MySQL server. Required. Type=int
- `_database`: The name of the database. Required. Type=string
- `_duration_ms`: The duration of the SQL execution in milliseconds. Optional, can be added by logging system if not provided. Type=int
- `_sql`: The actual SQL statement being executed. Optional. Truncated if there are more than 1024 UTF-8 characters.
- `_is_slow_query`: The flag indicating whether the SQL is a slow query. Optional, can be added by logging system if not provided.

Example log:
```json
{
    "_level": "info",
    "_server_ip": "192.168.0.102",
    "_server_port": 8081,
    "_service": "B",
    "_instance_ip": "192.168.0.104",
    "_instance_port": 3306,
    "_database": "example_db",
    "_sql": "SELECT * FROM users WHERE id = 1",
    "_duration_ms": 120,
    "_env": "prod",
    "_is_slow_query": false,
    "_msg": "SQL",
    "_trace_id": "1a2b3c4d5e6f7g8h9i0j",
    "_span_id": "1234567890abcdef",
    "_parent_span_id": "",
    "_ts": 1771521635000
}
```
