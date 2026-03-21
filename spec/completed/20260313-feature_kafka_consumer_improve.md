# Goal

- Feature: Kafka consumer logging & tracing.

# Facts

- Existing Kafka support is limited to `kafka/consumer.go`.
- Existing tracing pattern for RPC is implemented in `interceptor/trace.go` via dedicated wrappers (`NewTraceInterceptor`, `NewTraceClientInterceptor`) that create spans and enrich `context.Context` with trace fields.
- Structured logs use `_`-prefixed fields and are emitted through `log.Logger(ctx)` / `log.GlobalLog()`.

# Scope

- In scope:
  - Kafka consumer wrapper refactor to support trace/log instrumentation.
- Out of scope:
  - Kafka admin APIs (topic creation, ACL management, offset reset tooling).
  - Example app / docker-compose integration unless needed for tests.

# Design

## 1. Trace model

- Consumer span kind: `consumer`.
- Tracer names:
  - consumer: USE ENV `APP_NAME`
- Propagation model:
  - store W3C trace context in Kafka message headers
  - use OpenTelemetry text-map propagation, adapted to Sarama record headers

Required span attributes:

- `kafka.topic = <topic>`
- `kafka.partition = <partition>` when known
- `kafka.op = consume`
- `kafka.offset = <offset>` when known
- `instance` or remote broker endpoint when available
- `consumer.group = <group_id>` for consumer spans when available
- `error.flag = true|false`
- `duration.ms = <duration>`

Notes:
- Consumer should extract the parent trace from message headers before starting the processing span.

## 2. Logging model

- Use existing `_`-prefixed logging convention.
- Kafka logs should be emitted through `log.Logger(ctx)` where possible so `_trace_id`, `_span_id`, and `_parent_span_id` are inherited automatically.

Required Kafka log fields:

- `_topic`: Kafka topic. Required. Type=string
- `_partition`: Kafka partition. Required. Type=int
- `_instance_addr`: Kafka broker IP \ domain used by the client. Required when available; zero-value empty string is acceptable if Sarama does not expose a resolved broker address for the event. Type=string
- `_instance_port`: Kafka broker port used by the client. Optional when `_instance_addr` is not available; otherwise required when `_instance_addr` is present. Type=int
- `_dur_ms`: End-to-end publish or message handling duration in milliseconds. Required. Type=int

Recommended additional Kafka fields:

- `_offset`: Kafka offset. Required for consumer once message is read. Type=int64
- `_consumer_group`: Consumer group id. Required for consumer logs. Type=string
- `_consumer`: Logical consumer wrapper name. Optional. Type=string
- `_kafka_key`: Message key as string if safely printable, otherwise omit. Optional. Type=string
- `_error`: Error string on failures. Optional. Type=string

Message conventions:

- Consumer success log message: `KAFKA_CONSUME`
- Consumer handler error log message: `KAFKA_CONSUME`

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
  "_instance_addr": "10.42.0.31", // or "kafka-broker-1.internal:9092"
  "_dur_ms": 9,
  "_consumer_group": "billing-worker",
  "_consumer": "order-created-handler",
  "_app": "worker-service",
  "_ts": 1772509886138
}
```

## 3. Broker/IP handling

- Practical rule:
  - For consumer logs, `_instance_addr` should be the broker IP:Port or domain that served the fetch when available.
  - If Sarama does not expose this cleanly on the relevant path, log the resolved broker host string if available; otherwise inject empty string and document that limitation.

Recommended implementation approach:

- Start with a best-effort resolver from broker connection metadata already available through Sarama.
- Do not add fragile reflection or transport hacks just to force `_instance_addr`.

## 4. Error semantics

- Consumer:
  - Handler error marks span status as error and emits `_error` log field.
  - Whether offset is marked on handler error should remain under existing session/commit policy; this feature must not silently change acknowledgment semantics.

## 5. Acknowledgment semantics

- The consumer wrapper must never call `session.MarkMessage`.
- The message handler remains the sole owner of acknowledgment / commit behavior.
- Trace and log instrumentation must wrap message handling without changing when or whether the handler acknowledges the message.
- On handler error, the wrapper records the error in tracing/logging only; it must not add implicit acknowledge, retry, or commit behavior.
- If panic handling is added in the future, it must preserve the same rule: instrumentation may record the failure, but must not change acknowledgment semantics.

# Metric

New metric: 
- `kafka_consume.duration_ms`: Histogram of end-to-end message processing latency in milliseconds
- `kafka_consume.error_count`: Counter of message processing errors, labeled by error type

Labels: `kafka_instance, topic, partition, consumer_group, app`

# Verification
 