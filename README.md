# Project introduction

Lunaship is a microservices framework written in Go that provides basic observability and Kubernetes infrastructure integration.

Lunaship use `connect-go` as RPC framework. 

# Codebase structure

- `example/`: demo that based on lunaship, with deployment and test scripts.
- `infra/`: utils or wrapper that make logic more clear. 
- `utils/`: miscellaneous. 
- `interceptor/`: connect-go related interceptor, such as logging, tracing, metrics, etc.
- `kafka/`: Consumer & producer wrapper. 
- `log/`: Log initialization and related utils.
- `mysql/`: MySQL initialization, client and logger. 
- `redis/`: Redis client, logger and related utils.
- `setup/`: Lunaship service entrance. Every service should call `svc.Run()` to setup the server. 
- `spec/`: Specification driven development for vide coding. 
- `tests/: Integration tests for lunaship, including test cases and test data.

# Spec driven development

## Specification management
- `spec/progressing.md` for features\fix that are currently being worked on.
- `spec/archived/` for features\fix that are no longer being worked on, but may be useful for reference in the future.
- `spec/completed/` for features\fix that have been completed and are ready for use.

## Coding guidelines

- `spec/code_style.md` for code style and best practices.
- `spec/sql_standard.md` for SQL writing standard.