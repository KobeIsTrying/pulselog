package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pulselog/pulselog/internal/auth"
	"github.com/pulselog/pulselog/internal/config"
	"github.com/pulselog/pulselog/internal/identity"
	"github.com/pulselog/pulselog/internal/ratelimit"
)

var (
	testProjectA = uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	testProjectB = uuid.MustParse("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb")
	testOrgA     = uuid.MustParse("cccccccc-cccc-4ccc-8ccc-cccccccccccc")
)

type stubStore struct {
	pingErr error
	list    []LogRow
	cursor  string
	more    bool
	listErr error
	got     *LogRow
	getErr  error
	ov      *Overview
	ovErr   error
	ts      []TimeBucket
	tsErr   error
	svcs    []ServiceStat
	svcErr  error
	errs    []ErrorGroup
	errErr  error
	lastQ   listQuery
	lastTR  timeRange
}

func (s *stubStore) Ping(context.Context) error { return s.pingErr }

func (s *stubStore) ListLogs(_ context.Context, q listQuery) ([]LogRow, string, bool, error) {
	s.lastQ = q
	return s.list, s.cursor, s.more, s.listErr
}

func (s *stubStore) GetLog(_ context.Context, _ uuid.UUID, _ []uuid.UUID) (*LogRow, error) {
	return s.got, s.getErr
}

func (s *stubStore) Overview(_ context.Context, r timeRange) (*Overview, error) {
	s.lastTR = r
	if s.ov == nil {
		return &Overview{}, nil
	}
	return s.ov, s.ovErr
}

func (s *stubStore) Timeseries(context.Context, timeseriesQuery) ([]TimeBucket, error) {
	return s.ts, s.tsErr
}

func (s *stubStore) Services(context.Context, serviceStatsQuery) ([]ServiceStat, error) {
	return s.svcs, s.svcErr
}

func (s *stubStore) TopErrors(context.Context, errorStatsQuery) ([]ErrorGroup, error) {
	return s.errs, s.errErr
}

func ownerPrincipal() *identity.Principal {
	return &identity.Principal{
		UserID:     uuid.MustParse("11111111-1111-4111-8111-111111111111"),
		Email:      "a@example.com",
		ProjectIDs: []uuid.UUID{testProjectA},
		OrgRoles:   map[uuid.UUID]auth.Role{testOrgA: auth.RoleOwner},
	}
}

func testServer(store Store) *Server {
	s := NewServer(slog.New(slog.NewTextHandler(io.Discard, nil)), store, time.Second)
	s.authn = func(*http.Request) (*identity.Principal, error) { return ownerPrincipal(), nil }
	return s
}

func testHandler(store Store) http.Handler {
	return testServer(store).Handler()
}

func do(h http.Handler, method, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func TestListLogsOKAndFilters(t *testing.T) {
	st := &stubStore{
		list: []LogRow{{EventID: "11111111-1111-4111-8111-111111111111", Service: "payment-service", Level: "ERROR", Message: "x"}},
		more: true, cursor: "abc",
	}
	rec := do(testHandler(st), http.MethodGet, "/api/v1/logs?service=payment-service&level=ERROR&page_size=10")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if st.lastQ.Service != "payment-service" || st.lastQ.Level != "ERROR" || st.lastQ.Limit != 10 {
		t.Fatalf("query %+v", st.lastQ)
	}
	var body listResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.HasMore || body.NextCursor != "abc" || len(body.Logs) != 1 {
		t.Fatalf("%+v", body)
	}
}

func TestListLogsValidationErrors(t *testing.T) {
	h := testHandler(&stubStore{})
	cases := []string{
		"/api/v1/logs?level=TRACE",
		"/api/v1/logs?page_size=500",
		"/api/v1/logs?cursor=not-a-cursor",
		"/api/v1/logs?start=yesterday",
		"/api/v1/logs?event_id=nope",
	}
	for _, p := range cases {
		rec := do(h, http.MethodGet, p)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s status=%d body=%s", p, rec.Code, rec.Body.String())
		}
	}
}

func TestListLogsPaginationCursor(t *testing.T) {
	ts := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	id := "11111111-1111-4111-8111-111111111111"
	cur := encodeCursor(ts, id)
	st := &stubStore{list: []LogRow{}}
	rec := do(testHandler(st), http.MethodGet, "/api/v1/logs?page_size=2&cursor="+cur)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if st.lastQ.Cursor == nil || st.lastQ.Cursor.ID != id || !st.lastQ.Cursor.TS.Equal(ts) {
		t.Fatalf("cursor %+v", st.lastQ.Cursor)
	}
	var body listResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Logs == nil || body.HasMore || body.PageSize != 2 {
		t.Fatalf("%+v", body)
	}
}

func TestGetLogFoundAndNotFoundAndInvalidID(t *testing.T) {
	id := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	h := testHandler(&stubStore{got: &LogRow{EventID: id.String(), Message: "hi"}})
	rec := do(h, http.MethodGet, "/api/v1/logs/"+id.String())
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	h = testHandler(&stubStore{getErr: errNotFound})
	rec = do(h, http.MethodGet, "/api/v1/logs/"+id.String())
	if rec.Code != http.StatusNotFound {
		t.Fatalf("%d", rec.Code)
	}
	rec = do(h, http.MethodGet, "/api/v1/logs/not-uuid")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("%d", rec.Code)
	}
}

func TestStatsEndpoints(t *testing.T) {
	h := testHandler(&stubStore{
		ov:   &Overview{Total: 10, Error: 2, ErrorRate: 0.2, ActiveServices: 3},
		ts:   []TimeBucket{{Count: 4}},
		svcs: []ServiceStat{{Service: "api", Total: 5, Errors: 1}},
		errs: []ErrorGroup{{Message: "boom", Count: 2}},
	})
	if do(h, http.MethodGet, "/api/v1/stats/overview").Code != 200 {
		t.Fatal("overview")
	}
	if do(h, http.MethodGet, "/api/v1/stats/timeseries?interval=1m").Code != 200 {
		t.Fatal("timeseries")
	}
	if do(h, http.MethodGet, "/api/v1/stats/services?sort=error_count").Code != 200 {
		t.Fatal("services")
	}
	if do(h, http.MethodGet, "/api/v1/stats/errors").Code != 200 {
		t.Fatal("errors")
	}
	if do(h, http.MethodGet, "/api/v1/stats/timeseries?interval=bad").Code != 400 {
		t.Fatal("bad interval")
	}
}

func TestClickHouseUnavailableAndTimeout(t *testing.T) {
	h := testHandler(&stubStore{pingErr: errors.New("down")})
	rec := do(h, http.MethodGet, "/readyz")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("readyz %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "password") {
		t.Fatal("leaked secret")
	}
	h = testHandler(&stubStore{listErr: errors.New("clickhouse exploded SELECT *")})
	rec = do(h, http.MethodGet, "/api/v1/logs")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "SELECT") {
		t.Fatal("leaked sql")
	}
	h = testHandler(&stubStore{listErr: context.DeadlineExceeded})
	rec = do(h, http.MethodGet, "/api/v1/logs")
	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("%d", rec.Code)
	}
}

func TestHealthz(t *testing.T) {
	rec := do(testHandler(&stubStore{}), http.MethodGet, "/healthz")
	if rec.Code != 200 {
		t.Fatal(rec.Code)
	}
}

func TestQueryRequiresAuth(t *testing.T) {
	s := NewServer(slog.New(slog.NewTextHandler(io.Discard, nil)), &stubStore{}, time.Second)
	tok, err := auth.NewIssuer("unit-test-secret", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	s.tokens = tok
	rec := do(s.Handler(), http.MethodGet, "/api/v1/logs")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
}

func TestLoginMissingAndInvalidJSON(t *testing.T) {
	s := NewServer(slog.New(slog.NewTextHandler(io.Discard, nil)), &stubStore{}, time.Second)
	s.signups = true
	h := s.Handler()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("%d", rec.Code)
	}
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"a@example.com","password":"nope"}`))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable && rec.Code != http.StatusUnauthorized {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
}

func TestOverviewHonorsProjectID(t *testing.T) {
	st := &stubStore{ov: &Overview{Total: 0}}
	s := testServer(st)
	s.authn = func(*http.Request) (*identity.Principal, error) {
		p := ownerPrincipal()
		p.ProjectIDs = []uuid.UUID{testProjectA, testProjectB}
		return p, nil
	}
	rec := do(s.Handler(), http.MethodGet, "/api/v1/stats/overview?project_id="+testProjectB.String())
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	if len(st.lastTR.ProjectIDs) != 1 || st.lastTR.ProjectIDs[0] != testProjectB {
		t.Fatalf("project scope = %v, want [%s]", st.lastTR.ProjectIDs, testProjectB)
	}
}

func TestCrossProjectForbidden(t *testing.T) {
	st := &stubStore{}
	s := testServer(st)
	s.authn = func(*http.Request) (*identity.Principal, error) {
		return &identity.Principal{
			UserID:     uuid.MustParse("22222222-2222-4222-8222-222222222222"),
			Email:      "b@example.com",
			ProjectIDs: []uuid.UUID{testProjectB},
			OrgRoles:   map[uuid.UUID]auth.Role{},
		}, nil
	}
	rec := do(s.Handler(), http.MethodGet, "/api/v1/logs?project_id="+testProjectA.String())
	if rec.Code != http.StatusForbidden {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
}

func TestViewerCannotCreateProject(t *testing.T) {
	s := testServer(&stubStore{})
	s.authn = func(*http.Request) (*identity.Principal, error) {
		return &identity.Principal{
			UserID:     uuid.MustParse("33333333-3333-4333-8333-333333333333"),
			Email:      "v@example.com",
			ProjectIDs: []uuid.UUID{testProjectA},
			OrgRoles:   map[uuid.UUID]auth.Role{testOrgA: auth.RoleViewer},
		}, nil
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orgs/"+testOrgA.String()+"/projects", strings.NewReader(`{"name":"other"}`))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
}

func TestLoginRateLimit(t *testing.T) {
	s := NewServer(slog.New(slog.NewTextHandler(io.Discard, nil)), &stubStore{}, time.Second)
	s.limit = ratelimit.NewMemory()
	s.rate = config.RateLimitConfig{LoginLimit: 1, LoginWindow: time.Minute}
	h := s.Handler()
	body := `{"email":"a@example.com","password":"wrong-password"}`
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body)))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body)))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
}
