package models

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	MaxServiceLen    = 128
	MaxMessageLen    = 8192
	MaxHostLen       = 253
	MaxTraceIDLen    = 128
	MaxMetadataKeys  = 32
	MaxMetadataKey   = 64
	MaxMetadataValue = 1024
	MaxBatchEvents   = 500
)

var (
	servicePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
	allowedLevels  = map[string]struct{}{
		"DEBUG": {},
		"INFO":  {},
		"WARN":  {},
		"ERROR": {},
		"FATAL": {},
	}
)

// LogEvent is the canonical ingest payload and Kafka value.
type LogEvent struct {
	EventID   string            `json:"event_id"`
	ProjectID string            `json:"project_id,omitempty"`
	Service   string            `json:"service"`
	Level     string            `json:"level"`
	Message   string            `json:"message"`
	Timestamp time.Time         `json:"timestamp"`
	Host      string            `json:"host,omitempty"`
	TraceID   string            `json:"traceId,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type ValidationError struct {
	Fields []FieldError `json:"fields"`
}

func (e *ValidationError) Error() string {
	if e == nil || len(e.Fields) == 0 {
		return "validation failed"
	}
	return fmt.Sprintf("validation failed: %s %s", e.Fields[0].Field, e.Fields[0].Message)
}

func (e *ValidationError) append(field, message string) {
	e.Fields = append(e.Fields, FieldError{Field: field, Message: message})
}

// Normalize trims fields, uppercases level, defaults timestamp, and assigns an event id.
func (e *LogEvent) Normalize(now time.Time) {
	e.Service = strings.TrimSpace(e.Service)
	e.Level = strings.ToUpper(strings.TrimSpace(e.Level))
	e.Message = strings.TrimSpace(e.Message)
	e.Host = strings.TrimSpace(e.Host)
	e.TraceID = strings.TrimSpace(e.TraceID)
	e.EventID = strings.TrimSpace(e.EventID)
	e.ProjectID = strings.TrimSpace(e.ProjectID)

	if e.Timestamp.IsZero() {
		e.Timestamp = now.UTC()
	} else {
		e.Timestamp = e.Timestamp.UTC()
	}
	if e.EventID == "" {
		e.EventID = uuid.NewString()
	}
	if e.TraceID == "" && e.Metadata != nil {
		if v := strings.TrimSpace(e.Metadata["traceId"]); v != "" {
			e.TraceID = v
		} else if v := strings.TrimSpace(e.Metadata["trace_id"]); v != "" {
			e.TraceID = v
		}
	}
}

func IsAllowedLevel(level string) bool {
	_, ok := allowedLevels[strings.ToUpper(strings.TrimSpace(level))]
	return ok
}

func IsValidServiceName(service string) bool {
	return servicePattern.MatchString(service)
}

func (e LogEvent) Validate() error {
	ve := &ValidationError{}

	if e.Service == "" {
		ve.append("service", "is required")
	} else if len(e.Service) > MaxServiceLen || !servicePattern.MatchString(e.Service) {
		ve.append("service", "must be 1–128 characters matching [A-Za-z0-9][A-Za-z0-9._-]*")
	}

	if e.Level == "" {
		ve.append("level", "is required")
	} else if _, ok := allowedLevels[e.Level]; !ok {
		ve.append("level", "must be one of DEBUG, INFO, WARN, ERROR, FATAL")
	}

	if e.Message == "" {
		ve.append("message", "is required")
	} else if len(e.Message) > MaxMessageLen {
		ve.append("message", fmt.Sprintf("must be at most %d characters", MaxMessageLen))
	}

	if e.Host != "" && len(e.Host) > MaxHostLen {
		ve.append("host", fmt.Sprintf("must be at most %d characters", MaxHostLen))
	}
	if e.TraceID != "" && len(e.TraceID) > MaxTraceIDLen {
		ve.append("traceId", fmt.Sprintf("must be at most %d characters", MaxTraceIDLen))
	}
	if e.EventID != "" {
		if _, err := uuid.Parse(e.EventID); err != nil {
			ve.append("event_id", "must be a UUID")
		}
	}
	if e.ProjectID != "" {
		if _, err := uuid.Parse(e.ProjectID); err != nil {
			ve.append("project_id", "must be a UUID")
		}
	}
	if len(e.Metadata) > MaxMetadataKeys {
		ve.append("metadata", fmt.Sprintf("must have at most %d keys", MaxMetadataKeys))
	}
	for k, v := range e.Metadata {
		if k == "" || len(k) > MaxMetadataKey {
			ve.append("metadata", fmt.Sprintf("keys must be 1–%d characters", MaxMetadataKey))
			break
		}
		if len(v) > MaxMetadataValue {
			ve.append("metadata."+k, fmt.Sprintf("must be at most %d characters", MaxMetadataValue))
		}
	}

	if len(ve.Fields) > 0 {
		return ve
	}
	return nil
}

type BatchRequest struct {
	Events []LogEvent `json:"events"`
}

func (b BatchRequest) ValidateSize() error {
	ve := &ValidationError{}
	if len(b.Events) == 0 {
		ve.append("events", "must contain at least one event")
		return ve
	}
	if len(b.Events) > MaxBatchEvents {
		ve.append("events", fmt.Sprintf("must contain at most %d events", MaxBatchEvents))
		return ve
	}
	return nil
}

func ValidateBatch(events []LogEvent) error {
	ve := &ValidationError{}
	for i, ev := range events {
		err := ev.Validate()
		if err == nil {
			continue
		}
		var inner *ValidationError
		if ok := asValidation(err, &inner); ok {
			for _, f := range inner.Fields {
				ve.append(fmt.Sprintf("events[%d].%s", i, f.Field), f.Message)
			}
			continue
		}
		ve.append(fmt.Sprintf("events[%d]", i), err.Error())
	}
	if len(ve.Fields) > 0 {
		return ve
	}
	return nil
}

func asValidation(err error, target **ValidationError) bool {
	v, ok := err.(*ValidationError)
	if !ok {
		return false
	}
	*target = v
	return true
}
