package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	registerOnce sync.Once

	HTTPRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pulselog_http_requests_total",
			Help: "HTTP requests handled by the service.",
		},
		[]string{"method", "path", "status"},
	)

	HTTPDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "pulselog_http_request_duration_seconds",
			Help:    "HTTP request latency.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	EventsAccepted = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "pulselog_ingest_events_accepted_total",
			Help: "Log events accepted and published to Kafka.",
		},
	)

	EventsRejected = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pulselog_ingest_events_rejected_total",
			Help: "Log events rejected before Kafka publish.",
		},
		[]string{"reason"},
	)

	KafkaProduceDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "pulselog_kafka_produce_duration_seconds",
			Help:    "Time spent publishing a batch to Kafka.",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		},
	)

	KafkaProduceErrors = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "pulselog_kafka_produce_errors_total",
			Help: "Kafka produce failures.",
		},
	)

	ProcessorConsumed = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "pulselog_processor_events_consumed_total",
			Help: "Kafka records fetched by the log processor.",
		},
	)

	ProcessorWritten = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "pulselog_processor_events_written_total",
			Help: "Events successfully inserted into ClickHouse.",
		},
	)

	ProcessorFailed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pulselog_processor_events_failed_total",
			Help: "Events that failed processing.",
		},
		[]string{"reason"},
	)

	ProcessorRetried = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "pulselog_processor_events_retried_total",
			Help: "ClickHouse insert attempts after the first failure.",
		},
	)

	ProcessorDLQ = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pulselog_processor_events_dlq_total",
			Help: "Events published to the dead-letter topic.",
		},
		[]string{"reason"},
	)

	ClickHouseWriteDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "pulselog_processor_clickhouse_write_duration_seconds",
			Help:    "Time spent inserting a batch into ClickHouse.",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
	)

	ProcessorBatchSize = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "pulselog_processor_batch_size",
			Help:    "Number of Kafka records in a processed batch.",
			Buckets: []float64{1, 5, 10, 25, 50, 100, 250, 500},
		},
	)

	HTTPInFlight = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "pulselog_http_requests_in_flight",
			Help: "In-flight HTTP requests.",
		},
	)

	ClickHouseQueryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "pulselog_clickhouse_query_duration_seconds",
			Help:    "Time spent running a ClickHouse read query.",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"op"},
	)

	ClickHouseQueryErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pulselog_clickhouse_query_errors_total",
			Help: "ClickHouse read query failures.",
		},
		[]string{"op"},
	)

	AuthSuccess = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pulselog_auth_success_total",
			Help: "Successful authentications.",
		},
		[]string{"kind"},
	)

	AuthFailure = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pulselog_auth_failures_total",
			Help: "Failed authentications.",
		},
		[]string{"kind", "reason"},
	)

	APIKeyRejected = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pulselog_api_key_rejected_total",
			Help: "Ingest requests rejected because of an API key.",
		},
		[]string{"reason"},
	)

	RateLimited = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pulselog_rate_limited_total",
			Help: "Requests rejected by rate limiting.",
		},
		[]string{"scope"},
	)

	AuthzDenied = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pulselog_authz_denied_total",
			Help: "Authorization denials.",
		},
		[]string{"reason"},
	)

	RealtimePublished = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "pulselog_realtime_published_total",
			Help: "Realtime payloads published to Redis after ClickHouse write.",
		},
	)

	RealtimePublishErrors = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "pulselog_realtime_publish_errors_total",
			Help: "Failed Redis realtime publishes.",
		},
	)

	RedisSubscribeErrors = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "pulselog_realtime_subscribe_errors_total",
			Help: "Redis pub/sub subscriber failures.",
		},
	)

	WSConnections = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "pulselog_ws_connections",
			Help: "Active authenticated WebSocket connections.",
		},
	)

	WSConnects = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "pulselog_ws_connects_total",
			Help: "Successful WebSocket upgrades.",
		},
	)

	WSAuthFailures = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "pulselog_ws_auth_failures_total",
			Help: "Rejected WebSocket authentication attempts.",
		},
	)

	WSMessagesDelivered = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "pulselog_ws_messages_delivered_total",
			Help: "Realtime frames queued to a subscriber.",
		},
	)

	WSDisconnects = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "pulselog_ws_disconnects_total",
			Help: "WebSocket connections closed.",
		},
	)

	WSMessagesDropped = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "pulselog_ws_messages_dropped_total",
			Help: "Realtime frames dropped because a subscriber buffer was full.",
		},
	)

	KafkaConsumerLag = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "pulselog_kafka_consumer_lag",
			Help: "Approximate Kafka consumer lag reported by the processor reader.",
		},
	)
)

func Register() {
	registerOnce.Do(func() {
		prometheus.MustRegister(
			HTTPRequests,
			HTTPDuration,
			EventsAccepted,
			EventsRejected,
			KafkaProduceDuration,
			KafkaProduceErrors,
			ProcessorConsumed,
			ProcessorWritten,
			ProcessorFailed,
			ProcessorRetried,
			ProcessorDLQ,
			ClickHouseWriteDuration,
			ProcessorBatchSize,
			HTTPInFlight,
			ClickHouseQueryDuration,
			ClickHouseQueryErrors,
			AuthSuccess,
			AuthFailure,
			APIKeyRejected,
			RateLimited,
			AuthzDenied,
			RealtimePublished,
			RealtimePublishErrors,
			RedisSubscribeErrors,
			WSConnections,
			WSConnects,
			WSAuthFailures,
			WSMessagesDelivered,
			WSDisconnects,
			WSMessagesDropped,
			KafkaConsumerLag,
		)
	})
}
