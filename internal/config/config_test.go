package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("KAFKA_BROKERS", "")
	os.Unsetenv("KAFKA_BROKERS")
	os.Unsetenv("HTTP_ADDR")
	os.Unsetenv("KAFKA_TOPIC_INGEST")

	cfg, err := Load("ingestion-api")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr = %q", cfg.HTTPAddr)
	}
	if got, want := cfg.Kafka.Brokers, []string{"localhost:9092"}; len(got) != 1 || got[0] != want[0] {
		t.Fatalf("Brokers = %#v", got)
	}
	if cfg.Kafka.WriteTimeout != 5*time.Second {
		t.Fatalf("WriteTimeout = %s", cfg.Kafka.WriteTimeout)
	}
}

func TestLoadKafkaBrokersCSV(t *testing.T) {
	t.Setenv("KAFKA_BROKERS", "kafka-1:9092, kafka-2:9092")
	cfg, err := Load("ingestion-api")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Kafka.Brokers) != 2 {
		t.Fatalf("len(Brokers) = %d", len(cfg.Kafka.Brokers))
	}
}

func TestLoadProcessorHTTPAddr(t *testing.T) {
	t.Setenv("PROCESSOR_HTTP_ADDR", "")
	os.Unsetenv("PROCESSOR_HTTP_ADDR")
	os.Unsetenv("KAFKA_TOPIC_INGEST")
	os.Unsetenv("KAFKA_TOPIC_DLQ")
	cfg, err := Load("log-processor")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPAddr != ":8081" {
		t.Fatalf("HTTPAddr = %q", cfg.HTTPAddr)
	}
	if cfg.Kafka.TopicDLQ != "logs-dlq" {
		t.Fatalf("TopicDLQ = %q", cfg.Kafka.TopicDLQ)
	}
	if cfg.Kafka.TopicIngest != "logs-ingest" {
		t.Fatalf("TopicIngest = %q", cfg.Kafka.TopicIngest)
	}
	if cfg.Kafka.ConsumerGroup != "log-processor" {
		t.Fatalf("ConsumerGroup = %q", cfg.Kafka.ConsumerGroup)
	}
	if cfg.Processor.MaxAttempts != 5 {
		t.Fatalf("MaxAttempts = %d", cfg.Processor.MaxAttempts)
	}
}

func TestLoadQueryHTTPAddr(t *testing.T) {
	os.Unsetenv("QUERY_HTTP_ADDR")
	os.Unsetenv("QUERY_TIMEOUT")
	cfg, err := Load("query-api")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPAddr != ":8082" {
		t.Fatalf("HTTPAddr = %q", cfg.HTTPAddr)
	}
	if cfg.Query.Timeout != 5*time.Second {
		t.Fatalf("Query.Timeout = %s", cfg.Query.Timeout)
	}
	if cfg.Query.WSBuffer != 256 {
		t.Fatalf("Query.WSBuffer = %d", cfg.Query.WSBuffer)
	}
}

func TestLoadProductionSignupsAndRetention(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("JWT_SECRET", "unit-test-production-secret")
	t.Setenv("AUTH_SIGNUPS", "")
	os.Unsetenv("AUTH_SIGNUPS")
	t.Setenv("CLICKHOUSE_TTL_DAYS", "30")
	cfg, err := Load("query-api")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Auth.AllowSignup {
		t.Fatal("production must default AUTH_SIGNUPS to false")
	}
	if cfg.ClickHouse.TTLDays != 30 {
		t.Fatalf("TTLDays = %d", cfg.ClickHouse.TTLDays)
	}
	if cfg.RateLimit.IngestLimit != 120 {
		t.Fatalf("production ingest rate default must stay 120, got %d", cfg.RateLimit.IngestLimit)
	}
}

func TestLoadProductionRequiresJWTSecret(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("JWT_SECRET", "")
	os.Unsetenv("JWT_SECRET")
	if _, err := Load("query-api"); err == nil {
		t.Fatal("expected JWT_SECRET required in production")
	}
}

func TestLoadInvalidTimeout(t *testing.T) {
	t.Setenv("KAFKA_WRITE_TIMEOUT", "not-a-duration")
	if _, err := Load("ingestion-api"); err == nil {
		t.Fatal("expected error")
	}
}
