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

- Dynamic listening and reloading. 

# Design 

## Initialization

We assume that `config/app.toml` is already mounted to the application container before startup (like configmap in k8s, or bind mount in docker).

The `config/app.toml` is loaded and fixed at startup. We don't support dynamic reloading for now.

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

# Required. The unique name of the application.
app_name = "logs-demo"

[app.log]
# Optional. The commit hash of the application. Default: "error"
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

# Deployment

We need a configmap that contains the `app.toml` file for each application. The configmap can be generated from a template file with environment variable substitution.

## Example: app_config.yaml
todo 