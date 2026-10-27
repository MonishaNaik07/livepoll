package websocket

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/your-username/live-polling-app/backend/internal/apperr"
	rds "github.com/your-username/live-polling-app/backend/internal/redis"
	"github.com/your-username/live-polling-app/backend/internal/services"
	"github.com/your-username/live-polling-app/backend/internal/validation"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingEvery  = 25 * time.Second
	maxReadLen = 512
	sendBuffer = 32
)

// Hub holds this instance's WebSocket clients, grouped per poll. One Redis pattern subscription per
// instance feeds it, so votes accepted by ANY backend instance reach clients on THIS instance.
type Hub struct {
	store    *rds.Store
	polls    *services.PollService
	log      *slog.Logger
	origin   string
	mu       sync.RWMutex
	rooms    map[string]map[*client]struct{}
	upgrader websocket.Upgrader
}

type client struct {
	hub    *Hub
	conn   *websocket.Conn
	pollID string
	send   chan []byte
	done   chan struct{}
	once   sync.Once
}

func NewHub(store *rds.Store, polls *services.PollService, log *slog.Logger, allowedOrigin string) *Hub {
	h := &Hub{store: store, polls: polls, log: log, origin: allowedOrigin, rooms: map[string]map[*client]struct{}{}}
	h.upgrader = websocket.Upgrader{
		ReadBufferSize: 1024, WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			o := r.Header.Get("Origin")
			return o == "" || o == allowedOrigin // browsers always send Origin; reject foreign sites
		},
	}
	return h
}

type frame struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

func encode(t string, data []byte) []byte {
	b, _ := json.Marshal(frame{Type: t, Data: data})
	return b
}

// Run consumes Redis Pub/Sub until ctx is cancelled. go-redis reconnects automatically; events
// missed during a Redis outage are healed by clients re-fetching a snapshot (see frontend).
func (h *Hub) Run(ctx context.Context) {
	ps := h.store.PSubscribe(ctx)
	defer ps.Close()
	ch := ps.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case m, ok := <-ch:
			if !ok {
				return
			}
			h.dispatch(strings.TrimPrefix(m.Channel, rds.ChannelPrefix), encode("update", []byte(m.Payload)))
		}
	}
}

func (h *Hub) dispatch(pollID string, msg []byte) {
	var slow []*client
	h.mu.RLock()
	for c := range h.rooms[pollID] {
		select {
		case c.send <- msg:
		default:
			slow = append(slow, c) // never block the hub on one slow consumer
		}
	}
	h.mu.RUnlock()
	for _, c := range slow {
		c.close()
	}
}

func (h *Hub) add(c *client) {
	h.mu.Lock()
	if h.rooms[c.pollID] == nil {
		h.rooms[c.pollID] = map[*client]struct{}{}
	}
	h.rooms[c.pollID][c] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) remove(c *client) {
	h.mu.Lock()
	if room := h.rooms[c.pollID]; room != nil {
		delete(room, c)
		if len(room) == 0 {
			delete(h.rooms, c.pollID)
		}
	}
	h.mu.Unlock()
	c.close()
}

func (c *client) close() {
	c.once.Do(func() {
		close(c.done)
		_ = c.conn.Close()
	})
}

// Shutdown closes every connection (hijacked conns are not closed by http.Server.Shutdown).
func (h *Hub) Shutdown() {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, room := range h.rooms {
		for c := range room {
			c.close()
		}
	}
}

// Serve handles GET /api/ws/polls/:id. Public poll data only; no secrets cross this socket.
func (h *Hub) Serve(c *gin.Context) {
	id, e := validation.ParseObjectID(c.Param("id"))
	if e != nil {
		c.JSON(e.Status, gin.H{"error": e})
		return
	}
	p, err := h.polls.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(apperr.ErrNotFound.Status, gin.H{"error": apperr.ErrNotFound})
		return
	}
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return // Upgrade already wrote the HTTP error
	}
	cl := &client{hub: h, conn: conn, pollID: id.Hex(), send: make(chan []byte, sendBuffer), done: make(chan struct{})}
	h.add(cl) // register BEFORE snapshot so no event can fall in the gap; clients order by seq
	go cl.writePump()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	res, err := h.polls.Results(ctx, p)
	cancel()
	if err != nil {
		h.log.Warn("ws snapshot failed", "poll", cl.pollID, "err", err)
		h.remove(cl)
		return
	}
	b, _ := json.Marshal(res)
	select {
	case cl.send <- encode("snapshot", b):
	default:
	}
	cl.readPump()
}

// readPump discards (and size-limits) client frames; it exists to detect disconnects and keepalive pongs.
func (c *client) readPump() {
	defer c.hub.remove(c)
	c.conn.SetReadLimit(maxReadLen)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	}
}

// writePump is the ONLY writer to the connection (gorilla requires a single concurrent writer).
func (c *client) writePump() {
	t := time.NewTicker(pingEvery)
	defer t.Stop()
	defer c.hub.remove(c)
	for {
		select {
		case <-c.done:
			return
		case msg := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-t.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

