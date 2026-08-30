package clickhouse

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/pulselog/pulselog/internal/config"
)

var identRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)?$`)

type Client struct {
	Conn  driver.Conn
	Table string
}

func openConn(cfg config.ClickHouseConfig) (driver.Conn, string, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{cfg.Addr},
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.User,
			Password: cfg.Password,
		},
		DialTimeout:     5 * time.Second,
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 30 * time.Minute,
		Compression:     &clickhouse.Compression{Method: clickhouse.CompressionLZ4},
	})
	if err != nil {
		return nil, "", fmt.Errorf("clickhouse open: %w", err)
	}
	table := cfg.Table
	if table == "" {
		table = "logs"
	}
	if cfg.Database != "" {
		table = cfg.Database + "." + table
	}
	if !identRE.MatchString(table) {
		return nil, "", fmt.Errorf("invalid clickhouse table identifier")
	}
	return conn, table, nil
}

func OpenClient(cfg config.ClickHouseConfig) (*Client, error) {
	conn, table, err := openConn(cfg)
	if err != nil {
		return nil, err
	}
	return &Client{Conn: conn, Table: table}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	if c == nil || c.Conn == nil {
		return fmt.Errorf("clickhouse not connected")
	}
	return c.Conn.Ping(ctx)
}

func (c *Client) Close() error {
	if c == nil || c.Conn == nil {
		return nil
	}
	return c.Conn.Close()
}

func (c *Client) EnsureProjectColumn(ctx context.Context) error {
	if c == nil || c.Conn == nil {
		return fmt.Errorf("clickhouse not connected")
	}
	return c.Conn.Exec(ctx, fmt.Sprintf(
		`ALTER TABLE %s ADD COLUMN IF NOT EXISTS project_id UUID DEFAULT toUUID('00000000-0000-0000-0000-000000000000')`,
		c.Table,
	))
}
