package clickhouse

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/uuid"
	"github.com/pulselog/pulselog/internal/config"
	"github.com/pulselog/pulselog/internal/metrics"
	"github.com/pulselog/pulselog/internal/models"
)

// Writer inserts canonical log events into ClickHouse.
type Writer struct {
	conn  driver.Conn
	table string
}

func Open(cfg config.ClickHouseConfig) (*Writer, error) {
	conn, table, err := openConn(cfg)
	if err != nil {
		return nil, err
	}
	return &Writer{conn: conn, table: table}, nil
}

func (w *Writer) Ping(ctx context.Context) error {
	if w == nil || w.conn == nil {
		return fmt.Errorf("clickhouse not connected")
	}
	return w.conn.Ping(ctx)
}

func (w *Writer) Insert(ctx context.Context, events []models.LogEvent) error {
	if len(events) == 0 {
		return nil
	}
	start := time.Now()
	err := w.insert(ctx, events)
	metrics.ClickHouseWriteDuration.Observe(time.Since(start).Seconds())
	if err != nil {
		return err
	}
	return nil
}

func (w *Writer) insert(ctx context.Context, events []models.LogEvent) error {
	q := fmt.Sprintf(`INSERT INTO %s (event_id, timestamp, ingested_at, service, level, message, host, trace_id, metadata, project_id)`, w.table)
	batch, err := w.conn.PrepareBatch(ctx, q)
	if err != nil {
		return fmt.Errorf("clickhouse prepare: %w", err)
	}
	ingestedAt := time.Now().UTC()
	for _, ev := range events {
		id, err := uuid.Parse(ev.EventID)
		if err != nil {
			return fmt.Errorf("event_id %q: %w", ev.EventID, err)
		}
		meta := ev.Metadata
		if meta == nil {
			meta = map[string]string{}
		}
		projectID := uuid.Nil
		if ev.ProjectID != "" {
			projectID, err = uuid.Parse(ev.ProjectID)
			if err != nil {
				return fmt.Errorf("project_id %q: %w", ev.ProjectID, err)
			}
		}
		if err := batch.Append(
			id,
			ev.Timestamp.UTC(),
			ingestedAt,
			ev.Service,
			ev.Level,
			ev.Message,
			ev.Host,
			ev.TraceID,
			meta,
			projectID,
		); err != nil {
			return fmt.Errorf("clickhouse append: %w", err)
		}
	}
	if err := batch.Send(); err != nil {
		return fmt.Errorf("clickhouse send: %w", err)
	}
	return nil
}

func (w *Writer) Close() error {
	if w == nil || w.conn == nil {
		return nil
	}
	return w.conn.Close()
}

func (w *Writer) EnsureProjectColumn(ctx context.Context) error {
	if w == nil || w.conn == nil {
		return fmt.Errorf("clickhouse not connected")
	}
	return w.conn.Exec(ctx, fmt.Sprintf(
		`ALTER TABLE %s ADD COLUMN IF NOT EXISTS project_id UUID DEFAULT toUUID('00000000-0000-0000-0000-000000000000')`,
		w.table,
	))
}
