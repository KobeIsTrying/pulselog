package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is process configuration loaded from the environment.
type Config struct {
	Env          string
	LogLevel     string
	ServiceName  string
	HTTPAddr     string
	MaxBodyBytes int64
	Kafka        KafkaConfig
	ClickHouse   ClickHouseConfig
	Processor    ProcessorConfig
	Query        QueryConfig
	Postgres     PostgresConfig
	Redis        RedisConfig
	Auth         AuthConfig
	RateLimit    RateLimitConfig
}

type KafkaConfig struct {
	Brokers       []string
	TopicIngest   string
	TopicDLQ      string
	ConsumerGroup string
	WriteTimeout  time.Duration
}

type ClickHouseConfig struct {
	Addr      string
	Database  string
	User      string
	Password  string
	Table     string
	TTLDays   int
	MVTTLDays int
}

type QueryConfig struct {
	Timeout  time.Duration
	WSBuffer int
}

type PostgresConfig struct {
	DSN string
}

type RedisConfig struct {
	Addr string
}

type AuthConfig struct {
	JWTSecret   string
	JWTTTL      time.Duration
	AllowSignup bool
	CORSOrigins []string
}

type RateLimitConfig struct {
	LoginLimit   int
	LoginWindow  time.Duration
	IngestLimit  int
	IngestWindow time.Duration
	QueryLimit   int
	QueryWindow  time.Duration
}

type ProcessorConfig struct {
	BatchSize     int
	BatchTimeout  time.Duration
	MaxAttempts   int
	RetryBackoff  time.Duration
	WriteTimeout  time.Duration
	ShutdownGrace time.Duration
}

func Load(serviceName string) (Config, error) {
	writeTimeout, err := parseDuration("KAFKA_WRITE_TIMEOUT", 5*time.Second)
	if err != nil {
		return Config{}, err
	}
	maxBody, err := parseInt64("HTTP_MAX_BODY_BYTES", 5<<20)
	if err != nil {
		return Config{}, err
	}
	batchTimeout, err := parseDuration("PROCESSOR_BATCH_TIMEOUT", 500*time.Millisecond)
	if err != nil {
		return Config{}, err
	}
	retryBackoff, err := parseDuration("PROCESSOR_RETRY_BACKOFF", 200*time.Millisecond)
	if err != nil {
		return Config{}, err
	}
	chTimeout, err := parseDuration("CLICKHOUSE_WRITE_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}
	shutdownGrace, err := parseDuration("PROCESSOR_SHUTDOWN_GRACE", 15*time.Second)
	if err != nil {
		return Config{}, err
	}
	batchSize, err := parseInt("PROCESSOR_BATCH_SIZE", 100)
	if err != nil {
		return Config{}, err
	}
	maxAttempts, err := parseInt("PROCESSOR_MAX_ATTEMPTS", 5)
	if err != nil {
		return Config{}, err
	}

	httpAddr := getenv("HTTP_ADDR", ":8080")
	if serviceName == "log-processor" {
		httpAddr = getenv("PROCESSOR_HTTP_ADDR", ":8081")
	}
	if serviceName == "query-api" {
		httpAddr = getenv("QUERY_HTTP_ADDR", ":8082")
	}

	queryTimeout, err := parseDuration("QUERY_TIMEOUT", 5*time.Second)
	if err != nil {
		return Config{}, err
	}
	jwtTTL, err := parseDuration("JWT_TTL", 24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	loginWindow, err := parseDuration("RATE_LIMIT_LOGIN_WINDOW", time.Minute)
	if err != nil {
		return Config{}, err
	}
	ingestWindow, err := parseDuration("RATE_LIMIT_INGEST_WINDOW", time.Minute)
	if err != nil {
		return Config{}, err
	}
	queryWindow, err := parseDuration("RATE_LIMIT_QUERY_WINDOW", time.Minute)
	if err != nil {
		return Config{}, err
	}
	loginLimit, err := parseInt("RATE_LIMIT_LOGIN", 10)
	if err != nil {
		return Config{}, err
	}
	ingestLimit, err := parseInt("RATE_LIMIT_INGEST", 120)
	if err != nil {
		return Config{}, err
	}
	queryLimit, err := parseInt("RATE_LIMIT_QUERY", 120)
	if err != nil {
		return Config{}, err
	}
	wsBuffer, err := parseInt("WS_HUB_BUFFER", 256)
	if err != nil {
		return Config{}, err
	}
	if wsBuffer < 1 {
		wsBuffer = 256
	}
	ttlDays, err := parseInt("CLICKHOUSE_TTL_DAYS", 90)
	if err != nil {
		return Config{}, err
	}
	if ttlDays < 1 {
		ttlDays = 90
	}
	mvTTLDays, err := parseInt("CLICKHOUSE_MV_TTL_DAYS", 180)
	if err != nil {
		return Config{}, err
	}
	if mvTTLDays < 1 {
		mvTTLDays = 180
	}

	env := getenv("ENV", "development")
	jwtSecret := getenv("JWT_SECRET", "")
	if jwtSecret == "" && env != "production" {
		jwtSecret = "pulselog_dev_jwt_only"
	}
	if (serviceName == "query-api") && jwtSecret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET is required")
	}

	cfg := Config{
		Env:          getenv("ENV", "development"),
		LogLevel:     getenv("LOG_LEVEL", "info"),
		ServiceName:  serviceName,
		HTTPAddr:     httpAddr,
		MaxBodyBytes: maxBody,
		Kafka: KafkaConfig{
			Brokers:       splitCSV(getenv("KAFKA_BROKERS", "localhost:9092")),
			TopicIngest:   getenv("KAFKA_TOPIC_INGEST", "logs-ingest"),
			TopicDLQ:      getenv("KAFKA_TOPIC_DLQ", "logs-dlq"),
			ConsumerGroup: getenv("KAFKA_CONSUMER_GROUP", "log-processor"),
			WriteTimeout:  writeTimeout,
		},
		ClickHouse: ClickHouseConfig{
			Addr:      getenv("CLICKHOUSE_ADDR", "localhost:9000"),
			Database:  getenv("CLICKHOUSE_DATABASE", "pulselog"),
			User:      getenv("CLICKHOUSE_USER", "pulselog"),
			Password:  getenv("CLICKHOUSE_PASSWORD", "pulselog_dev_only"),
			Table:     getenv("CLICKHOUSE_TABLE", "logs"),
			TTLDays:   ttlDays,
			MVTTLDays: mvTTLDays,
		},
		Processor: ProcessorConfig{
			BatchSize:     batchSize,
			BatchTimeout:  batchTimeout,
			MaxAttempts:   maxAttempts,
			RetryBackoff:  retryBackoff,
			WriteTimeout:  chTimeout,
			ShutdownGrace: shutdownGrace,
		},
		Query: QueryConfig{
			Timeout:  queryTimeout,
			WSBuffer: wsBuffer,
		},
		Postgres: PostgresConfig{
			DSN: getenv("POSTGRES_DSN", "postgres://pulselog:pulselog_dev_only@localhost:5432/pulselog?sslmode=disable"),
		},
		Redis: RedisConfig{
			Addr: getenv("REDIS_ADDR", "localhost:6379"),
		},
		Auth: AuthConfig{
			JWTSecret:   jwtSecret,
			JWTTTL:      jwtTTL,
			AllowSignup: parseBool("AUTH_SIGNUPS", env != "production"),
			CORSOrigins: splitCSV(getenv("CORS_ORIGINS", "")),
		},
		RateLimit: RateLimitConfig{
			LoginLimit:   loginLimit,
			LoginWindow:  loginWindow,
			IngestLimit:  ingestLimit,
			IngestWindow: ingestWindow,
			QueryLimit:   queryLimit,
			QueryWindow:  queryWindow,
		},
	}
	if len(cfg.Kafka.Brokers) == 0 {
		return Config{}, fmt.Errorf("KAFKA_BROKERS must not be empty")
	}
	if cfg.Kafka.TopicIngest == "" {
		return Config{}, fmt.Errorf("KAFKA_TOPIC_INGEST must not be empty")
	}
	if cfg.Processor.BatchSize < 1 {
		return Config{}, fmt.Errorf("PROCESSOR_BATCH_SIZE must be >= 1")
	}
	if cfg.Processor.MaxAttempts < 1 {
		return Config{}, fmt.Errorf("PROCESSOR_MAX_ATTEMPTS must be >= 1")
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return fallback
}

func splitCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseDuration(key string, fallback time.Duration) (time.Duration, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	return d, nil
}

func parseInt64(key string, fallback int64) (int64, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	return n, nil
}

func parseInt(key string, fallback int) (int, error) {
	n, err := parseInt64(key, int64(fallback))
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

func parseBool(key string, fallback bool) bool {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback
	}
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}
