package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// ParseJSON unmarshals a Kafka payload into a LogEvent, then normalizes it.
// JSON errors are returned as-is. Validation errors are *ValidationError.
func ParseJSON(raw []byte, now time.Time) (LogEvent, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return LogEvent{}, fmt.Errorf("empty payload")
	}
	var ev LogEvent
	if err := json.Unmarshal(raw, &ev); err != nil {
		return LogEvent{}, err
	}
	ev.Normalize(now)
	if err := ev.Validate(); err != nil {
		return ev, err
	}
	return ev, nil
}
