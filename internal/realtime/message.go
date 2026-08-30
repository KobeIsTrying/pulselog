package realtime

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/pulselog/pulselog/internal/models"
)

const (
	SchemaVersion  = 1
	TypeLogCreated = "log.created"
	TypeHello      = "hello"
	TypeError      = "error"
	ChannelPrefix  = "pulselog:logs:"
	MaxPayload     = 16 << 10
)

type Envelope struct {
	V         int     `json:"v"`
	Type      string  `json:"type"`
	ProjectID string  `json:"project_id,omitempty"`
	Data      LogData `json:"data,omitempty"`
}

type LogData struct {
	EventID   string            `json:"event_id"`
	ProjectID string            `json:"project_id,omitempty"`
	Service   string            `json:"service"`
	Level     string            `json:"level"`
	Message   string            `json:"message"`
	Timestamp string            `json:"timestamp"`
	Host      string            `json:"host,omitempty"`
	TraceID   string            `json:"trace_id,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

func Channel(projectID string) string {
	return ChannelPrefix + strings.TrimSpace(projectID)
}

func ProjectFromChannel(channel string) string {
	return strings.TrimPrefix(channel, ChannelPrefix)
}

func EncodeEvent(ev models.LogEvent) ([]byte, error) {
	ts := ev.Timestamp.UTC().Format(time.RFC3339Nano)
	if ev.Timestamp.IsZero() {
		ts = time.Now().UTC().Format(time.RFC3339Nano)
	}
	env := Envelope{
		V:    SchemaVersion,
		Type: TypeLogCreated,
		Data: LogData{
			EventID:   ev.EventID,
			ProjectID: ev.ProjectID,
			Service:   ev.Service,
			Level:     ev.Level,
			Message:   ev.Message,
			Timestamp: ts,
			Host:      ev.Host,
			TraceID:   ev.TraceID,
			Metadata:  ev.Metadata,
		},
	}
	b, err := json.Marshal(env)
	if err != nil {
		return nil, err
	}
	if len(b) > MaxPayload {
		env.Data.Message = truncate(env.Data.Message, 2048)
		env.Data.Metadata = nil
		b, err = json.Marshal(env)
	}
	return b, err
}

func DecodeEnvelope(raw []byte) (Envelope, error) {
	var env Envelope
	err := json.Unmarshal(raw, &env)
	return env, err
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
