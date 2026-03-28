# Goal

- Add `app.toml` for application initialization.
- Put all Redis, MySQL, Kafka config into one place. 
- Allow custom config with auto serialization and validation.

# Background

We use environment variables for configuration and there are some issues:
- Env definition are scattered in different places, hard to maintain.
- Ugly deserialization when passing a structured config through env vars.

# Out of scope

- Dynamic listening and reloading. 

# Design 

## Initialization & validate once

The `app.toml` is loaded and fixed at startup. We don't support dynamic reloading for now.

If the config is invalid, such as lack of `APP_NAME`, the application will print the error log and then panic. 

## Environment variables

We still need env vars for runtime overrides, such as `ENV`, `COMMIT_HASH`. 

## Config struct

Key parts:
- Service metadata like `APP_NAME`, `COMMIT_HASH`, `LOG_LEVEL`
- DB & middleware endpoint with multiple instances support, e.g. `REDIS_URLS`, `MYSQL_URLS`, `KAFKA_BROKERS`.
- Other custom config. 

## Config example:

```toml
[app]
app_name = "logs-demo"
# env = "test" # default, may be covered by env "ENV"
# commit_hash = "ad0433faaba5c50c8da870293eec11a204b61d6e"
log_level = "info"
trace_sample_strategy = "default"

[app.log]
level = "info" # debug, info, warn, error. Default is "info".

[redis] # global config for all Redis instances

[redis.dev-redis] # An endpoint named "dev-redis"
addr = "redis.tinyinfra.dev"
port = 6379

[mysql] # global config for all MySQL instances
max_open_conns = 300
max_idle_conns = 100
max_idle_time = "30s"

[mysql.dev-mysql] # An endpoint named "dev-mysql"
addr = "mysql.tinyinfra.dev"
port = 3306
username = "root"
password = "password"
database = "test" # optional. 

# optional, default is false. Ping the MySQL server at startup to validate the connection. 
# Panic if ping failed. 
ping_enabled = true

[kafka] # global config for all Kafka instances
[kafka.producer.dev] # An producer endpoint named "producer.dev"
brokers = ["kafka.tinyinfra.dev:9092"]
topic = "test-topic"
acks = "all" # default is "all". Other options: "0", "1".

[kafka.consumer.dev] # An consumer endpoint named "consumer.dev"
brokers = ["kafka.tinyinfra.dev:9092"]
topic = "test-topic"
acks = "all" # default is "all". Other options: "0", "1".

[module-keyvalue] # custom config for a module named "keyvalue"
key1 = "value1"
key2 = "value2"
```

## Deserialization & validation

We use custom go struct for custom config deserialization and validation. For example, the `kafka` config can be deserialized into a struct like:

```go
type ModuleKeyValue struct {
    Key1 string `toml:"key1"`
    Key2 string `toml:"key2"`
}

func FromAppToml(body []byte) (*ModuleKeyValue, error) {
    var config ModuleKeyValue
    if err := UnmarshalToml(body, ".module-keyvalue", &config); err != nil {
        log.Error("failed to unmarshal app.toml", "error", err)
        return nil, err
    }
    return &config, nil
}
```

