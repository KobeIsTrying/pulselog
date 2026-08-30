package httpx

import (
	"bufio"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pulselog/pulselog/internal/metrics"
)

type Middleware func(http.Handler) http.Handler

func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

func Recover(log *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Error("panic recovered", "panic", rec, "path", r.URL.Path)
					http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r)
	})
}

func AccessLog(log *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)
			if r.URL.Path == "/metrics" || r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
				return
			}
			log.Info("http",
				"method", r.Method,
				"path", r.URL.Path,
				"status", sw.status,
				"bytes", sw.bytes,
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", w.Header().Get("X-Request-ID"),
			)
		})
	}
}

func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}
		metrics.HTTPInFlight.Inc()
		defer metrics.HTTPInFlight.Dec()
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		path := routeLabel(r.URL.Path)
		metrics.HTTPRequests.WithLabelValues(r.Method, path, strconv.Itoa(sw.status)).Inc()
		metrics.HTTPDuration.WithLabelValues(r.Method, path).Observe(time.Since(start).Seconds())
	})
}

func routeLabel(path string) string {
	switch path {
	case "/v1/logs", "/v1/logs/batch", "/ingest", "/healthz", "/readyz",
		"/api/v1/logs", "/api/v1/stats/overview", "/api/v1/stats/timeseries",
		"/api/v1/stats/services", "/api/v1/stats/errors",
		"/api/v1/auth/register", "/api/v1/auth/login", "/api/v1/auth/logout", "/api/v1/auth/me",
		"/api/v1/stream", "/api/v1/stream/ticket":
		return path
	default:
		if strings.HasPrefix(path, "/api/v1/logs/") {
			return "/api/v1/logs/:event_id"
		}
		if strings.HasPrefix(path, "/api/v1/orgs/") && strings.Contains(path, "/members") {
			return "/api/v1/orgs/:id/members"
		}
		if strings.HasPrefix(path, "/api/v1/orgs/") && strings.Contains(path, "/projects") {
			return "/api/v1/orgs/:id/projects"
		}
		if strings.HasPrefix(path, "/api/v1/orgs") {
			return "/api/v1/orgs"
		}
		if strings.HasPrefix(path, "/api/v1/projects/") && strings.Contains(path, "/api-keys") {
			return "/api/v1/projects/:id/api-keys"
		}
		if strings.HasPrefix(path, "/api/v1/projects/") && strings.Contains(path, "/services") {
			return "/api/v1/projects/:id/services"
		}
		if strings.HasPrefix(path, "/api/v1/api-keys/") {
			return "/api/v1/api-keys/:id"
		}
		return "other"
	}
}

func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Permitted-Cross-Domain-Policies", "none")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func CORS(origins []string) Middleware {
	allowed := map[string]struct{}{}
	for _, o := range origins {
		o = strings.TrimSpace(o)
		if o != "" && o != "*" {
			allowed[o] = struct{}{}
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				if _, ok := allowed[origin]; ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Vary", "Origin")
					w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-API-Key, X-Request-ID")
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
				}
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(p []byte) (int, error) {
	n, err := w.ResponseWriter.Write(p)
	w.bytes += n
	return n, err
}

func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return h.Hijack()
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
