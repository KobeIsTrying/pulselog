package main

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pulselog/pulselog/internal/models"
)

const (
	defaultPageSize = 50
	maxPageSize     = 200
	maxSearchLen    = 256
	defaultStatsWin = 24 * time.Hour
)

type apiError struct {
	Status int
	Code   string
	Msg    string
	Fields []models.FieldError
}

func (e *apiError) Error() string { return e.Msg }

func badRequest(field, msg string) *apiError {
	return &apiError{
		Status: 400,
		Code:   "invalid_query",
		Msg:    "invalid query parameters",
		Fields: []models.FieldError{{Field: field, Message: msg}},
	}
}

type orderDir int

const (
	orderNewest orderDir = iota
	orderOldest
)

type pageCursor struct {
	TS time.Time
	ID string
}

func encodeCursor(ts time.Time, id string) string {
	raw := ts.UTC().Format(time.RFC3339Nano) + "|" + id
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(s string) (pageCursor, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return pageCursor{}, fmt.Errorf("malformed cursor")
	}
	parts := strings.SplitN(string(b), "|", 2)
	if len(parts) != 2 {
		return pageCursor{}, fmt.Errorf("malformed cursor")
	}
	ts, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		ts, err = time.Parse(time.RFC3339, parts[0])
		if err != nil {
			return pageCursor{}, fmt.Errorf("malformed cursor")
		}
	}
	if _, err := uuid.Parse(parts[1]); err != nil {
		return pageCursor{}, fmt.Errorf("malformed cursor")
	}
	return pageCursor{TS: ts.UTC(), ID: parts[1]}, nil
}

type listQuery struct {
	Service    string
	Level      string
	Start      *time.Time
	End        *time.Time
	Q          string
	EventID    string
	Limit      int
	Cursor     *pageCursor
	Order      orderDir
	ProjectIDs []uuid.UUID
}

type timeRange struct {
	Start      *time.Time
	End        *time.Time
	ProjectIDs []uuid.UUID
}

type timeseriesQuery struct {
	timeRange
	Interval string
	Service  string
	Level    string
}

type serviceStatsQuery struct {
	timeRange
	Sort string
}

type errorStatsQuery struct {
	timeRange
	Limit int
}

var allowedIntervals = map[string]string{
	"1m":  "toStartOfMinute(timestamp)",
	"5m":  "toStartOfFiveMinutes(timestamp)",
	"15m": "toStartOfFifteenMinutes(timestamp)",
	"1h":  "toStartOfHour(timestamp)",
	"1d":  "toStartOfDay(timestamp)",
}

var allowedServiceSort = map[string]string{
	"error_count": "errors DESC, total DESC",
	"total":       "total DESC, errors DESC",
	"error_rate":  "error_rate DESC, errors DESC",
}

func parseRFC3339(raw, field string) (*time.Time, *apiError) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	ts, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		ts, err = time.Parse(time.RFC3339, raw)
	}
	if err != nil {
		return nil, badRequest(field, "must be RFC3339")
	}
	utc := ts.UTC()
	return &utc, nil
}

func parseTimeRange(q url.Values, required bool) (timeRange, *apiError) {
	start, err := parseRFC3339(q.Get("start"), "start")
	if err != nil {
		return timeRange{}, err
	}
	end, err := parseRFC3339(q.Get("end"), "end")
	if err != nil {
		return timeRange{}, err
	}
	if start != nil && end != nil && !end.After(*start) {
		return timeRange{}, badRequest("end", "must be after start")
	}
	if required && start == nil && end == nil {
		now := time.Now().UTC()
		from := now.Add(-defaultStatsWin)
		return timeRange{Start: &from, End: &now}, nil
	}
	return timeRange{Start: start, End: end}, nil
}

func parseListQuery(q url.Values) (listQuery, *apiError) {
	out := listQuery{Limit: defaultPageSize, Order: orderNewest}
	if s := strings.TrimSpace(q.Get("service")); s != "" {
		if !models.IsValidServiceName(s) {
			return listQuery{}, badRequest("service", "invalid service name")
		}
		out.Service = s
	}
	if lvl := strings.TrimSpace(q.Get("level")); lvl != "" {
		lvl = strings.ToUpper(lvl)
		if !models.IsAllowedLevel(lvl) {
			return listQuery{}, badRequest("level", "must be one of DEBUG, INFO, WARN, ERROR, FATAL")
		}
		out.Level = lvl
	}
	start, err := parseRFC3339(q.Get("start"), "start")
	if err != nil {
		return listQuery{}, err
	}
	end, err := parseRFC3339(q.Get("end"), "end")
	if err != nil {
		return listQuery{}, err
	}
	if start != nil && end != nil && !end.After(*start) {
		return listQuery{}, badRequest("end", "must be after start")
	}
	if raw := strings.TrimSpace(q.Get("project_id")); raw != "" {
		id, perr := uuid.Parse(raw)
		if perr != nil {
			return listQuery{}, badRequest("project_id", "must be a UUID")
		}
		out.ProjectIDs = []uuid.UUID{id}
	}
	out.Start, out.End = start, end

	if msg := strings.TrimSpace(q.Get("q")); msg != "" {
		if len(msg) > maxSearchLen {
			return listQuery{}, badRequest("q", fmt.Sprintf("must be at most %d characters", maxSearchLen))
		}
		out.Q = msg
	}
	if id := strings.TrimSpace(q.Get("event_id")); id != "" {
		if _, perr := uuid.Parse(id); perr != nil {
			return listQuery{}, badRequest("event_id", "must be a UUID")
		}
		out.EventID = id
	}
	if raw := strings.TrimSpace(q.Get("page_size")); raw != "" {
		n, perr := strconv.Atoi(raw)
		if perr != nil || n < 1 {
			return listQuery{}, badRequest("page_size", "must be an integer >= 1")
		}
		if n > maxPageSize {
			return listQuery{}, badRequest("page_size", fmt.Sprintf("must be at most %d", maxPageSize))
		}
		out.Limit = n
	}
	if cur := strings.TrimSpace(q.Get("cursor")); cur != "" {
		pc, cerr := decodeCursor(cur)
		if cerr != nil {
			return listQuery{}, badRequest("cursor", "is invalid")
		}
		out.Cursor = &pc
	}
	switch strings.ToLower(strings.TrimSpace(q.Get("order"))) {
	case "", "newest":
		out.Order = orderNewest
	case "oldest":
		out.Order = orderOldest
	default:
		return listQuery{}, badRequest("order", "must be newest or oldest")
	}
	return out, nil
}

func parseRequestedProjects(q url.Values) ([]uuid.UUID, *apiError) {
	raw := strings.TrimSpace(q.Get("project_id"))
	if raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil, badRequest("project_id", "must be a UUID")
	}
	return []uuid.UUID{id}, nil
}

func parseTimeseriesQuery(q url.Values) (timeseriesQuery, *apiError) {
	tr, err := parseTimeRange(q, true)
	if err != nil {
		return timeseriesQuery{}, err
	}
	interval := strings.TrimSpace(q.Get("interval"))
	if interval == "" {
		interval = "1m"
	}
	if _, ok := allowedIntervals[interval]; !ok {
		return timeseriesQuery{}, badRequest("interval", "must be one of 1m, 5m, 15m, 1h, 1d")
	}
	out := timeseriesQuery{timeRange: tr, Interval: interval}
	if s := strings.TrimSpace(q.Get("service")); s != "" {
		if !models.IsValidServiceName(s) {
			return timeseriesQuery{}, badRequest("service", "invalid service name")
		}
		out.Service = s
	}
	if lvl := strings.TrimSpace(q.Get("level")); lvl != "" {
		lvl = strings.ToUpper(lvl)
		if !models.IsAllowedLevel(lvl) {
			return timeseriesQuery{}, badRequest("level", "must be one of DEBUG, INFO, WARN, ERROR, FATAL")
		}
		out.Level = lvl
	}
	return out, nil
}

func parseServiceStatsQuery(q url.Values) (serviceStatsQuery, *apiError) {
	tr, err := parseTimeRange(q, true)
	if err != nil {
		return serviceStatsQuery{}, err
	}
	sort := strings.TrimSpace(q.Get("sort"))
	if sort == "" {
		sort = "error_count"
	}
	if _, ok := allowedServiceSort[sort]; !ok {
		return serviceStatsQuery{}, badRequest("sort", "must be one of error_count, total, error_rate")
	}
	return serviceStatsQuery{timeRange: tr, Sort: sort}, nil
}

func parseErrorStatsQuery(q url.Values) (errorStatsQuery, *apiError) {
	tr, err := parseTimeRange(q, true)
	if err != nil {
		return errorStatsQuery{}, err
	}
	limit := 20
	if raw := strings.TrimSpace(q.Get("limit")); raw != "" {
		n, perr := strconv.Atoi(raw)
		if perr != nil || n < 1 || n > 50 {
			return errorStatsQuery{}, badRequest("limit", "must be an integer between 1 and 50")
		}
		limit = n
	}
	return errorStatsQuery{timeRange: tr, Limit: limit}, nil
}
