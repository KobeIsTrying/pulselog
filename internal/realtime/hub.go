package realtime

import (
	"sync"

	"github.com/pulselog/pulselog/internal/metrics"
)

type Client struct {
	Send      chan []byte
	ProjectID string
}

func NewClient(projectID string, buffer int) *Client {
	if buffer < 1 {
		buffer = 64
	}
	return &Client{Send: make(chan []byte, buffer), ProjectID: projectID}
}

type Hub struct {
	mu        sync.Mutex
	clients   map[*Client]struct{}
	byProject map[string]map[*Client]struct{}
}

func NewHub() *Hub {
	return &Hub{
		clients:   map[*Client]struct{}{},
		byProject: map[string]map[*Client]struct{}{},
	}
}

func (h *Hub) Add(c *Client) {
	if h == nil || c == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c] = struct{}{}
	if h.byProject[c.ProjectID] == nil {
		h.byProject[c.ProjectID] = map[*Client]struct{}{}
	}
	h.byProject[c.ProjectID][c] = struct{}{}
	metrics.WSConnections.Set(float64(len(h.clients)))
}

func (h *Hub) Remove(c *Client) {
	if h == nil || c == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, c)
	if set := h.byProject[c.ProjectID]; set != nil {
		delete(set, c)
		if len(set) == 0 {
			delete(h.byProject, c.ProjectID)
		}
	}
	metrics.WSConnections.Set(float64(len(h.clients)))
	metrics.WSDisconnects.Inc()
}

func (h *Hub) Deliver(projectID string, payload []byte) int {
	if h == nil || projectID == "" {
		return 0
	}
	h.mu.Lock()
	set := h.byProject[projectID]
	targets := make([]*Client, 0, len(set))
	for c := range set {
		targets = append(targets, c)
	}
	h.mu.Unlock()

	n := 0
	for _, c := range targets {
		select {
		case c.Send <- payload:
			n++
			metrics.WSMessagesDelivered.Inc()
		default:
			metrics.WSMessagesDropped.Inc()
		}
	}
	return n
}

func (h *Hub) Count() int {
	if h == nil {
		return 0
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}
