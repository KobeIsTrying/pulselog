package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	ch "github.com/pulselog/pulselog/internal/clickhouse"
	"github.com/pulselog/pulselog/internal/metrics"
)

var errNotFound = errors.New("not found")

type LogRow struct {
	EventID    string            `json:"event_id"`
	ProjectID  string            `json:"project_id,omitempty"`
	Timestamp  time.Time         `json:"timestamp"`
	IngestedAt time.Time         `json:"ingested_at"`
	Service    string            `json:"service"`
	Level      string            `json:"level"`
	Message    string            `json:"message"`
	Host       string            `json:"host,omitempty"`
	TraceID    string            `json:"trace_id,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type Overview struct {
	Total          uint64  `json:"total"`
	Debug          uint64  `json:"debug"`
	Info           uint64  `json:"info"`
	Warn           uint64  `json:"warn"`
	Error          uint64  `json:"error"`
	Fatal          uint64  `json:"fatal"`
	ErrorRate      float64 `json:"error_rate"`
	ActiveServices uint64  `json:"active_services"`
}

type TimeBucket struct {
	Bucket  time.Time `json:"bucket"`
	Count   uint64    `json:"count"`
	Service string    `json:"service,omitempty"`
	Level   string    `json:"level,omitempty"`
}

type ServiceStat struct {
	Service   string  `json:"service"`
	Total     uint64  `json:"total"`
	Errors    uint64  `json:"error_count"`
	Warnings  uint64  `json:"warn_count"`
	ErrorRate float64 `json:"error_rate"`
}

type ErrorGroup struct {
	Message string `json:"message"`
	Count   uint64 `json:"count"`
}

type Store interface {
	Ping(ctx context.Context) error
	ListLogs(ctx context.Context, q listQuery) ([]LogRow, string, bool, error)
	GetLog(ctx context.Context, id uuid.UUID, projectIDs []uuid.UUID) (*LogRow, error)
	Overview(ctx context.Context, r timeRange) (*Overview, error)
	Timeseries(ctx context.Context, q timeseriesQuery) ([]TimeBucket, error)
	Services(ctx context.Context, q serviceStatsQuery) ([]ServiceStat, error)
	TopErrors(ctx context.Context, q errorStatsQuery) ([]ErrorGroup, error)
}

type chStore struct {
	client *ch.Client
}

func newCHStore(client *ch.Client) *chStore {
	return &chStore{client: client}
}

func (s *chStore) Ping(ctx context.Context) error {
	return s.client.Ping(ctx)
}

type sqlParts struct {
	conds []string
	args  []any
}

func (p *sqlParts) add(cond string, args ...any) {
	p.conds = append(p.conds, cond)
	p.args = append(p.args, args...)
}

func (p sqlParts) where() string {
	if len(p.conds) == 0 {
		return ""
	}
	return " WHERE " + strings.Join(p.conds, " AND ")
}

func addTimeRange(p *sqlParts, r timeRange) {
	if r.Start != nil {
		p.add("timestamp >= ?", *r.Start)
	}
	if r.End != nil {
		p.add("timestamp < ?", *r.End)
	}
}

func addProjectScope(p *sqlParts, ids []uuid.UUID) {
	if len(ids) == 0 {
		p.add("1 = 0")
		return
	}
	holders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		holders[i] = "?"
		args[i] = id
	}
	p.add("project_id IN ("+strings.Join(holders, ",")+")", args...)
}

func addListFilters(p *sqlParts, q listQuery) {
	if q.Service != "" {
		p.add("service = ?", q.Service)
	}
	if q.Level != "" {
		p.add("level = ?", q.Level)
	}
	addTimeRange(p, timeRange{Start: q.Start, End: q.End})
	if q.Q != "" {
		p.add("positionCaseInsensitive(message, ?) > 0", q.Q)
	}
	if q.EventID != "" {
		p.add("event_id = ?", q.EventID)
	}
	addProjectScope(p, q.ProjectIDs)
}

func (s *chStore) timed(ctx context.Context, op string, fn func(context.Context) error) error {
	start := time.Now()
	err := fn(ctx)
	metrics.ClickHouseQueryDuration.WithLabelValues(op).Observe(time.Since(start).Seconds())
	if err != nil && !errors.Is(err, errNotFound) {
		metrics.ClickHouseQueryErrors.WithLabelValues(op).Inc()
	}
	return err
}

func (s *chStore) ListLogs(ctx context.Context, q listQuery) ([]LogRow, string, bool, error) {
	var rows []LogRow
	var next string
	var more bool
	err := s.timed(ctx, "list", func(ctx context.Context) error {
		p := sqlParts{}
		addListFilters(&p, q)
		if q.Cursor != nil {
			if q.Order == orderOldest {
				p.add("(timestamp > ? OR (timestamp = ? AND event_id > ?))", q.Cursor.TS, q.Cursor.TS, q.Cursor.ID)
			} else {
				p.add("(timestamp < ? OR (timestamp = ? AND event_id < ?))", q.Cursor.TS, q.Cursor.TS, q.Cursor.ID)
			}
		}
		orderSQL := "timestamp DESC, event_id DESC"
		if q.Order == orderOldest {
			orderSQL = "timestamp ASC, event_id ASC"
		}
		query := fmt.Sprintf(
			`SELECT event_id, timestamp, ingested_at, service, level, message, host, trace_id, metadata, project_id FROM %s%s ORDER BY %s LIMIT ?`,
			s.client.Table, p.where(), orderSQL,
		)
		args := append(append([]any{}, p.args...), q.Limit+1)
		rs, err := s.client.Conn.Query(ctx, query, args...)
		if err != nil {
			return err
		}
		defer rs.Close()
		out := make([]LogRow, 0, q.Limit)
		for rs.Next() {
			row, err := scanLog(rs)
			if err != nil {
				return err
			}
			out = append(out, row)
		}
		if err := rs.Err(); err != nil {
			return err
		}
		if len(out) > q.Limit {
			more = true
			out = out[:q.Limit]
			last := out[len(out)-1]
			next = encodeCursor(last.Timestamp, last.EventID)
		}
		rows = out
		return nil
	})
	return rows, next, more, err
}

func (s *chStore) GetLog(ctx context.Context, id uuid.UUID, projectIDs []uuid.UUID) (*LogRow, error) {
	var found *LogRow
	err := s.timed(ctx, "get", func(ctx context.Context) error {
		p := sqlParts{}
		p.add("event_id = ?", id)
		addProjectScope(&p, projectIDs)
		query := fmt.Sprintf(
			`SELECT event_id, timestamp, ingested_at, service, level, message, host, trace_id, metadata, project_id FROM %s%s ORDER BY ingested_at DESC LIMIT 1`,
			s.client.Table, p.where(),
		)
		rs, err := s.client.Conn.Query(ctx, query, p.args...)
		if err != nil {
			return err
		}
		defer rs.Close()
		if !rs.Next() {
			if err := rs.Err(); err != nil {
				return err
			}
			return errNotFound
		}
		row, err := scanLog(rs)
		if err != nil {
			return err
		}
		found = &row
		return rs.Err()
	})
	return found, err
}

func (s *chStore) Overview(ctx context.Context, r timeRange) (*Overview, error) {
	var out Overview
	err := s.timed(ctx, "overview", func(ctx context.Context) error {
		p := sqlParts{}
		addTimeRange(&p, r)
		addProjectScope(&p, r.ProjectIDs)
		query := fmt.Sprintf(`
SELECT
    count() AS total,
    countIf(level = 'DEBUG') AS debug,
    countIf(level = 'INFO') AS info,
    countIf(level = 'WARN') AS warn,
    countIf(level = 'ERROR') AS error,
    countIf(level = 'FATAL') AS fatal,
    uniqExact(service) AS services
FROM %s%s`, s.client.Table, p.where())
		row := s.client.Conn.QueryRow(ctx, query, p.args...)
		if err := row.Scan(&out.Total, &out.Debug, &out.Info, &out.Warn, &out.Error, &out.Fatal, &out.ActiveServices); err != nil {
			return err
		}
		if out.Total > 0 {
			out.ErrorRate = float64(out.Error+out.Fatal) / float64(out.Total)
		}
		return nil
	})
	return &out, err
}

func (s *chStore) Timeseries(ctx context.Context, q timeseriesQuery) ([]TimeBucket, error) {
	bucketExpr, ok := allowedIntervals[q.Interval]
	if !ok {
		return nil, fmt.Errorf("unsupported interval")
	}
	var out []TimeBucket
	err := s.timed(ctx, "timeseries", func(ctx context.Context) error {
		p := sqlParts{}
		addTimeRange(&p, q.timeRange)
		addProjectScope(&p, q.ProjectIDs)
		if q.Service != "" {
			p.add("service = ?", q.Service)
		}
		if q.Level != "" {
			p.add("level = ?", q.Level)
		}
		query := fmt.Sprintf(
			`SELECT %s AS bucket, count() AS count FROM %s%s GROUP BY bucket ORDER BY bucket`,
			bucketExpr, s.client.Table, p.where(),
		)
		rs, err := s.client.Conn.Query(ctx, query, p.args...)
		if err != nil {
			return err
		}
		defer rs.Close()
		for rs.Next() {
			var b TimeBucket
			if err := rs.Scan(&b.Bucket, &b.Count); err != nil {
				return err
			}
			b.Service = q.Service
			b.Level = q.Level
			out = append(out, b)
		}
		return rs.Err()
	})
	return out, err
}

func (s *chStore) Services(ctx context.Context, q serviceStatsQuery) ([]ServiceStat, error) {
	orderSQL, ok := allowedServiceSort[q.Sort]
	if !ok {
		return nil, fmt.Errorf("unsupported sort")
	}
	var out []ServiceStat
	err := s.timed(ctx, "services", func(ctx context.Context) error {
		p := sqlParts{}
		addTimeRange(&p, q.timeRange)
		addProjectScope(&p, q.ProjectIDs)
		query := fmt.Sprintf(`
SELECT
    service,
    count() AS total,
    countIf(level = 'ERROR' OR level = 'FATAL') AS errors,
    countIf(level = 'WARN') AS warnings,
    if(count() = 0, 0, countIf(level = 'ERROR' OR level = 'FATAL') / count()) AS error_rate
FROM %s%s
GROUP BY service
ORDER BY %s
LIMIT 100`, s.client.Table, p.where(), orderSQL)
		rs, err := s.client.Conn.Query(ctx, query, p.args...)
		if err != nil {
			return err
		}
		defer rs.Close()
		for rs.Next() {
			var st ServiceStat
			if err := rs.Scan(&st.Service, &st.Total, &st.Errors, &st.Warnings, &st.ErrorRate); err != nil {
				return err
			}
			out = append(out, st)
		}
		return rs.Err()
	})
	return out, err
}

func (s *chStore) TopErrors(ctx context.Context, q errorStatsQuery) ([]ErrorGroup, error) {
	var out []ErrorGroup
	err := s.timed(ctx, "errors", func(ctx context.Context) error {
		p := sqlParts{}
		addTimeRange(&p, q.timeRange)
		addProjectScope(&p, q.ProjectIDs)
		p.add("level = ?", "ERROR")
		query := fmt.Sprintf(
			`SELECT message, count() AS count FROM %s%s GROUP BY message ORDER BY count DESC LIMIT ?`,
			s.client.Table, p.where(),
		)
		args := append(append([]any{}, p.args...), q.Limit)
		rs, err := s.client.Conn.Query(ctx, query, args...)
		if err != nil {
			return err
		}
		defer rs.Close()
		for rs.Next() {
			var g ErrorGroup
			if err := rs.Scan(&g.Message, &g.Count); err != nil {
				return err
			}
			out = append(out, g)
		}
		return rs.Err()
	})
	return out, err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanLog(rs rowScanner) (LogRow, error) {
	var (
		row       LogRow
		id        uuid.UUID
		ts        time.Time
		ing       time.Time
		meta      map[string]string
		projectID uuid.UUID
	)
	if err := rs.Scan(&id, &ts, &ing, &row.Service, &row.Level, &row.Message, &row.Host, &row.TraceID, &meta, &projectID); err != nil {
		return LogRow{}, err
	}
	row.EventID = id.String()
	row.ProjectID = projectID.String()
	row.Timestamp = ts.UTC()
	row.IngestedAt = ing.UTC()
	row.Metadata = meta
	return row, nil
}
