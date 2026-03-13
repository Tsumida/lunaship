# Goal

- Add first-class Kafka producer support so Lunaship users can publish messages without assembling Sarama primitives manually.
- Upgrade Kafka consumer support so both consumer and producer follow the same observability model already used by RPC, Redis, and MySQL: tracing plus structured logging.
- Keep the implementation maintainable by separating raw Sarama setup from Lunaship wrappers.

# Facts

- Existing Kafka support is limited to `kafka/consumer.go`.
- `kafka/consumer.go` already exposes a `KafkaConsumer`, `ConsumerWrapper`, and `DefaultConfig()`, but there is no producer abstraction and no structured trace/log wrapper.
- Existing tracing pattern for RPC is implemented in `interceptor/trace.go` via dedicated wrappers (`NewTraceInterceptor`, `NewTraceClientInterceptor`) that create spans and enrich `context.Context` with trace fields.
- Existing tracing + logging pattern for infrastructure clients is implemented separately per integration:
  - MySQL: `infra/mysql_logger.go`
  - Redis: `redis/hook.go`
- Structured logs use `_`-prefixed fields and are emitted through `log.Logger(ctx)` / `log.GlobalLog()`.

# Scope

- In scope:
  - Kafka producer abstraction and config entrypoints in `kafka/`.
  - Kafka consumer wrapper refactor to support trace/log instrumentation.
  - Producer and consumer trace/log wrappers with stable log fields.
  - Unit tests for config defaults, wrapper behavior, and logging/tracing field population.
- Out of scope:
  - Metrics collection for Kafka in this feature.
  - Exactly-once semantics or transactional producer support.
  - Kafka admin APIs (topic creation, ACL management, offset reset tooling).
  - Example app / docker-compose integration unless needed for tests.

# Design

## 1. Public API shape

- Keep low-level Sarama compatibility, but add Lunaship entrypoints that are easy to adopt.
- Suggested package surface:
  - `kafka.NewConfig(opts ...ConfigOption) *sarama.Config`
  - `kafka.NewProducer(cfg *sarama.Config, options ProducerOptions) (*Producer, error)`
  - `kafka.NewConsumer(cfg *sarama.Config, options ConsumerOptions) (*KafkaConsumer, error)`
  - `kafka.NewProducerWrapper(name string, handler ProduceHook) *ProducerWrapper`
  - `kafka.NewConsumerWrapper(name string, handler MsgHandlerFunc) *ConsumerWrapper`
- Keep `DefaultConfig()` for backward compatibility if already used externally, but internally treat it as a thin alias over the new constructor.

## 2. Config strategy

- Current `DefaultConfig()` mixes producer and consumer defaults but offers no clear extension path.
- Introduce explicit option structs so users can configure producer/consumer behavior without directly editing Sarama fields for common cases.

Suggested option groups:

- `CommonKafkaOptions`
  - `Brokers []string`
  - `ClientID string`
  - `Version string` or `sarama.KafkaVersion`
  - `TLS *tls.Config` or a simplified TLS option group if needed later
  - `SASL` options placeholder, even if not implemented in first pass
- `ProducerOptions`
  - `Topic string` as optional default topic
  - `RequiredAcks`
  - `RetryMax`
  - `Compression`
  - `Idempotent bool`
  - `Sync bool` or default to synchronous producer initially
- `ConsumerOptions`
  - `Group string`
  - `Topics []string`
  - `OffsetInitial`
  - `AutoCommit bool`

Recommended first implementation choice:

- Keep the first producer implementation synchronous (`sarama.SyncProducer`).
- Reason:
  - Easier error propagation.
  - Easier to attach a single publish span/log lifecycle.
  - Lower API complexity for first release.

## 3. Producer wrapper design

- Add a Lunaship `Producer` type that owns:
  - broker list
  - default topic (optional)
  - `sarama.SyncProducer`
  - logger/tracer helpers
- Suggested send API:
  - `SendMessage(ctx context.Context, msg *sarama.ProducerMessage) (partition int32, offset int64, err error)`
  - Optional convenience API later: `Send(ctx context.Context, topic string, key, value []byte, headers map[string]string) error`
- The wrapper should:
  - start a client span before publish
  - inject trace context into Kafka headers
  - emit structured success/error logs
  - return partition/offset so callers can use them if needed

## 4. Consumer wrapper design

- Keep `KafkaConsumer.Start(...)` as the main consume loop, but remove direct logging from raw message handling and move observability concerns into a wrapper/helper layer.
- `ConsumerWrapper.ConsumeClaim(...)` should:
  - derive message-scoped context from message headers
  - start a consumer span for each message
  - enrich logger context with Kafka fields
  - call the user handler with the enriched context or a message wrapper

Recommended API change:

- Replace the current handler type:
  - from: `type MsgHandlerFunc func(session sarama.ConsumerGroupSession, msg *sarama.ConsumerMessage) error`
  - to one of:
    - `type MsgHandlerFunc func(ctx context.Context, session sarama.ConsumerGroupSession, msg *sarama.ConsumerMessage) error`
    - or add a new handler type and preserve the old one with an adapter

Recommended choice:

- Introduce a new context-aware handler type and keep backward compatibility through an adapter.
- Reason:
  - Trace and log propagation requires `context.Context`.
  - This matches Lunaship patterns used in RPC/Redis/MySQL.

## 5. Trace model

- Producer span kind: `client`
- Consumer span kind: `server` for message processing lifecycle
- Tracer names:
  - producer: `lunaship/kafka/producer`
  - consumer: `lunaship/kafka/consumer`
- Propagation model:
  - store W3C trace context in Kafka message headers
  - use OpenTelemetry text-map propagation, adapted to Sarama record headers

Required span attributes:

- `messaging.system = kafka`
- `messaging.destination.name = <topic>`
- `messaging.destination.partition.id = <partition>` when known
- `messaging.operation = publish | process`
- `messaging.kafka.message.offset = <offset>` when known
- `net.peer.name` or remote broker endpoint when available
- `error.flag = true|false`
- `duration.ms = <duration>`

Notes:

- Producer may not know the final partition/offset until after send returns, so final attributes should be set before ending the span.
- Consumer should extract the parent trace from message headers before starting the processing span.

## 6. Logging model

- Use existing `_`-prefixed logging convention.
- Kafka logs should be emitted through `log.Logger(ctx)` where possible so `_trace_id`, `_span_id`, and `_parent_span_id` are inherited automatically.

Required Kafka log fields:

- `_topic`: Kafka topic. Required. Type=string
- `_partition`: Kafka partition. Required. Type=int
- `_instance_addr`: Kafka broker IP \ domain used by the client. Required when available; zero-value empty string is acceptable if Sarama does not expose a resolved broker address for the event. Type=string
- `_instance_port`: Kafka broker port used by the client. Optional when `_instance_addr` is not available; otherwise required when `_instance_addr` is present. Type=int
- `_dur_ms`: End-to-end publish or message handling duration in milliseconds. Required. Type=int

Recommended additional Kafka fields:

- `_offset`: Kafka offset. Optional for producer before ack, required for consumer once message is read. Type=int64
- `_consumer_group`: Consumer group id. Required for consumer logs. Type=string
- `_consumer`: Logical consumer wrapper name. Optional. Type=string
- `_producer`: Logical producer wrapper name. Optional. Type=string
- `_kafka_key`: Message key as string if safely printable, otherwise omit. Optional. Type=string
- `_error`: Error string on failures. Optional. Type=string

Message conventions:

- Producer success log message: `KAFKA_PRODUCE`
- Producer error log message: `KAFKA_PRODUCE`
- Consumer success log message: `KAFKA_CONSUME`
- Consumer handler error log message: `KAFKA_CONSUME`

Example producer success log:

```json
{
  "_level": "info",
  "_msg": "KAFKA_PRODUCE",
  "_trace_id": "fed7f32fb91b980af0803a4e6929325f",
  "_span_id": "1d8cff653776a550",
  "_topic": "order.created",
  "_partition": 3,
  "_offset": 1042,
  "_instance_addr": "10.42.0.31",
  "_dur_ms": 14,
  "_producer": "order-producer",
  "_app": "billing-service",
  "_ts": 1772509886123
}
```

Example consumer success log:

```json
{
  "_level": "info",
  "_msg": "KAFKA_CONSUME",
  "_trace_id": "fed7f32fb91b980af0803a4e6929325f",
  "_span_id": "6f85f3319dd4de28",
  "_parent_span_id": "1d8cff653776a550",
  "_topic": "order.created",
  "_partition": 3,
  "_offset": 1042,
  "_instance_addr": "10.42.0.31",
  "_dur_ms": 9,
  "_consumer_group": "billing-worker",
  "_consumer": "order-created-handler",
  "_app": "worker-service",
  "_ts": 1772509886138
}
```

## 7. Broker/IP handling

- The requested `_instance_addr` field is straightforward for Redis/MySQL because there is one configured endpoint, but Kafka is more dynamic.
- Practical rule:
  - For producer logs, `_instance_addr` should be the broker IP that acknowledged the write when available.
  - For consumer logs, `_instance_addr` should be the broker IP that served the fetch when available.
  - If Sarama does not expose this cleanly on the relevant path, log the resolved broker host string if available; otherwise inject empty string and document that limitation.

Recommended implementation approach:

- Start with a best-effort resolver from broker connection metadata already available through Sarama.
- Do not add fragile reflection or transport hacks just to force `_instance_addr`.

## 8. Error semantics

- Producer:
  - Publish error marks span status as error and emits `_error` log field.
- Consumer:
  - Handler error marks span status as error and emits `_error` log field.
  - Whether offset is marked on handler error should remain under existing session/commit policy; this feature must not silently change acknowledgment semantics.

## 9. Backward compatibility

- Do not break existing `KafkaConsumer.Start(...)` callers.
- Preserve `DefaultConfig()` as an alias or compatibility helper.
- If handler signature changes, provide an adapter for existing `MsgHandlerFunc` users.

# Plan

1. Add Kafka config constructor and option structs in `kafka/` while keeping `DefaultConfig()` compatibility.
2. Add a synchronous producer abstraction with a minimal send API.
3. Add trace propagation helpers for Kafka headers.
4. Refactor consumer wrapper so per-message processing uses context-aware tracing/logging.
5. Add producer and consumer log field builders to keep log schema stable.
6. Add unit tests for:
   - config defaults
   - trace header inject/extract
   - producer success/error logging behavior
   - consumer success/error logging behavior
   - backward compatibility adapter behavior

# Verification

## Automated tests

- Producer tests:
  - success path emits a client span and structured log with `_topic`, `_partition`, `_instance_addr`, `_dur_ms`
  - error path emits error status and `_error`
  - trace context is injected into Kafka headers
- Consumer tests:
  - incoming Kafka headers are extracted into processing context
  - processing span becomes child of producer span when headers exist
  - success path emits structured log with `_topic`, `_partition`, `_instance_addr`, `_dur_ms`
  - handler error emits error status and `_error`
- Config tests:
  - defaults keep current ack/retry/offset behavior unless explicitly overridden
  - compatibility helper still returns valid Sarama config

## Manual verification

- Start a local Kafka cluster for testing.
- Send one traced message through the Lunaship producer.
- Consume it through the Lunaship consumer wrapper.
- Verify:
  - logs contain `_dur_ms`, `_topic`, `_partition`, `_instance_addr`
  - consumer trace is linked to producer trace
  - handler failure path logs `_error` and preserves existing offset/commit semantics

# Acceptance criteria

- Users can create a Kafka producer without manually instantiating Sarama producer internals.
- Users can configure common producer/consumer settings through Lunaship-owned config entrypoints.
- Producer and consumer wrappers emit tracing spans and structured logs.
- Kafka logs include `_dur_ms`, `_topic`, `_partition`, `_instance_addr` according to the documented rules.
- Existing consumer startup path remains usable or has a compatibility adapter.
- Tests cover both normal and error flows.

# Risks and mitigations

- Risk: `_instance_addr` may not be reliably available for every producer/consumer event from Sarama.
  - Mitigation: define best-effort semantics and allow empty string when transport metadata is unavailable.
- Risk: changing consumer handler signature may break existing callers.
  - Mitigation: add adapter-based backward compatibility.
- Risk: adding producer abstraction that is too opinionated will make advanced Sarama usage harder.
  - Mitigation: keep wrapper minimal and allow access to the underlying `sarama.Config` / producer when necessary.
- Risk: trace propagation format becomes incompatible with non-Lunaship consumers.
  - Mitigation: use standard OpenTelemetry W3C propagation in Kafka headers rather than custom header formats.

# Open questions

- Whether Lunaship should expose only synchronous producer in v1, or provide async producer in a follow-up feature.
- Whether `_instance_addr` should allow broker hostname when IP resolution is not available.
- Whether Kafka metrics should be added immediately after this feature using the same wrapper boundaries.