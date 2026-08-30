package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/pulselog/pulselog/internal/auth"
	"github.com/pulselog/pulselog/internal/identity"
	"github.com/pulselog/pulselog/internal/metrics"
	"github.com/pulselog/pulselog/internal/realtime"
)

const (
	wsWriteWait  = 10 * time.Second
	wsPongWait   = 60 * time.Second
	wsPingPeriod = 20 * time.Second
	wsMaxConns   = 256
)

func (s *Server) ticketStore() realtime.TicketStore {
	if s.tickets != nil {
		return s.tickets
	}
	if s.rdb != nil {
		return realtime.RedisTickets{RDB: s.rdb}
	}
	return nil
}

func (s *Server) maxWS() int {
	if s.maxConns > 0 {
		return s.maxConns
	}
	return wsMaxConns
}

func (s *Server) originAllowed(origin string) bool {
	if origin == "" {
		return true
	}
	for _, o := range s.cors {
		if strings.TrimSpace(o) == origin {
			return true
		}
	}
	if s.env != "production" {
		switch origin {
		case "http://127.0.0.1:3000", "http://localhost:3000":
			return true
		}
	}
	return false
}

func (s *Server) handleStreamTicket(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	store := s.ticketStore()
	if store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error":   "unavailable",
			"message": "realtime tickets unavailable",
		})
		return
	}
	id, err := store.Issue(r.Context(), realtime.Ticket{
		UserID:     p.UserID,
		Email:      p.Email,
		JTI:        p.JTI,
		ProjectIDs: append([]uuid.UUID(nil), p.ProjectIDs...),
	}, 45*time.Second)
	if err != nil {
		s.log.Warn("stream ticket issue failed", "err", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error":   "unavailable",
			"message": "could not issue stream ticket",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ticket": id, "expires_in": 45})
}

func (s *Server) authenticateStream(r *http.Request) (*identity.Principal, error) {
	if ticket := strings.TrimSpace(r.URL.Query().Get("ticket")); ticket != "" {
		store := s.ticketStore()
		if store == nil {
			return nil, auth.ErrUnauthorized
		}
		t, err := store.Redeem(r.Context(), ticket)
		if err != nil {
			return nil, auth.ErrUnauthorized
		}
		if s.deny != nil && t.JTI != "" {
			denied, err := s.deny.Denied(r.Context(), t.JTI)
			if err != nil || denied {
				return nil, auth.ErrUnauthorized
			}
		}
		return &identity.Principal{
			UserID:     t.UserID,
			Email:      t.Email,
			JTI:        t.JTI,
			ProjectIDs: t.ProjectIDs,
		}, nil
	}
	return s.principal(r)
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	if s.limit != nil && !s.limit.Allow(r.Context(), "ws", auth.ClientIP(r), s.rate.QueryLimit, s.rate.QueryWindow) {
		metrics.WSAuthFailures.Inc()
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate_limited", "message": "too many requests"})
		return
	}
	p, err := s.authenticateStream(r)
	if err != nil {
		metrics.WSAuthFailures.Inc()
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized", "message": "authentication required"})
		return
	}
	projectID, err := uuid.Parse(strings.TrimSpace(r.URL.Query().Get("project_id")))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_query", "message": "project_id is required"})
		return
	}
	if !p.HasProject(projectID) {
		metrics.AuthzDenied.WithLabelValues("project").Inc()
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden", "message": "project is not accessible"})
		return
	}
	if s.hub == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unavailable", "message": "realtime hub unavailable"})
		return
	}
	if s.hub.Count() >= s.maxWS() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unavailable", "message": "too many live connections"})
		return
	}

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 4096,
		CheckOrigin: func(req *http.Request) bool {
			return s.originAllowed(req.Header.Get("Origin"))
		},
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.log.Warn("websocket upgrade failed", "err", err)
		return
	}
	metrics.WSConnects.Inc()

	buf := s.wsBuffer
	if buf < 1 {
		buf = 256
	}
	client := realtime.NewClient(projectID.String(), buf)
	s.hub.Add(client)
	defer func() {
		s.hub.Remove(client)
		_ = conn.Close()
	}()

	hello, _ := json.Marshal(realtime.Envelope{
		V:         realtime.SchemaVersion,
		Type:      realtime.TypeHello,
		ProjectID: projectID.String(),
	})
	_ = conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
	if err := conn.WriteMessage(websocket.TextMessage, hello); err != nil {
		return
	}

	conn.SetReadLimit(realtime.MaxPayload)
	_ = conn.SetReadDeadline(time.Now().Add(wsPongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(wsPongWait))
	})

	done := make(chan struct{})
	go s.wsWritePump(conn, client, p.JTI, done)
	s.wsReadPump(conn, done)
}

func (s *Server) wsReadPump(conn *websocket.Conn, done chan struct{}) {
	defer close(done)
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (s *Server) wsWritePump(conn *websocket.Conn, client *realtime.Client, jti string, done <-chan struct{}) {
	ticker := time.NewTicker(wsPingPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case payload, ok := <-client.Send:
			if !ok {
				return
			}
			_ = conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				return
			}
		case <-ticker.C:
			if jti != "" && s.deny != nil {
				denied, err := s.deny.Denied(context.Background(), jti)
				if err == nil && denied {
					_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "session ended"), time.Now().Add(wsWriteWait))
					return
				}
			}
			_ = conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
