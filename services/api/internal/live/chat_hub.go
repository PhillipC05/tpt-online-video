package live

// Chat hub: WebSocket room management and minimal RFC 6455 implementation.
// No external WebSocket library is required.

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

// ─── WebSocket wire constants ────────────────────────────────────────────────

const (
	wsGUID          = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
	wsOpcodeText    = 0x1
	wsOpcodeBinary  = 0x2
	wsOpcodeClose   = 0x8
	wsOpcodePing    = 0x9
	wsOpcodePong    = 0xA
	wsMaxFrameBytes = 64 * 1024 // 64 KB per frame
)

// wsAcceptKey computes the Sec-WebSocket-Accept header value.
func wsAcceptKey(clientKey string) string {
	h := sha1.New()
	h.Write([]byte(clientKey + wsGUID))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// WSConn wraps a hijacked net.Conn with buffered reading and WebSocket framing.
type WSConn struct {
	conn net.Conn
	r    *bufio.Reader
	wmu  sync.Mutex // serialise writes
}

// readFrame reads one complete WebSocket frame, returning opcode and unmasked payload.
func (c *WSConn) readFrame() (opcode byte, payload []byte, err error) {
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	header := make([]byte, 2)
	if _, err = io.ReadFull(c.r, header); err != nil {
		return
	}

	opcode = header[0] & 0x0F
	masked := header[1]>>7 == 1
	payloadLen := int64(header[1] & 0x7F)

	switch payloadLen {
	case 126:
		ext := make([]byte, 2)
		if _, err = io.ReadFull(c.r, ext); err != nil {
			return
		}
		payloadLen = int64(binary.BigEndian.Uint16(ext))
	case 127:
		ext := make([]byte, 8)
		if _, err = io.ReadFull(c.r, ext); err != nil {
			return
		}
		payloadLen = int64(binary.BigEndian.Uint64(ext))
	}

	if payloadLen > wsMaxFrameBytes {
		err = fmt.Errorf("websocket frame too large: %d bytes", payloadLen)
		return
	}

	var maskKey [4]byte
	if masked {
		if _, err = io.ReadFull(c.r, maskKey[:]); err != nil {
			return
		}
	}

	payload = make([]byte, payloadLen)
	if _, err = io.ReadFull(c.r, payload); err != nil {
		return
	}

	if masked {
		for i := range payload {
			payload[i] ^= maskKey[i%4]
		}
	}
	return
}

// writeFrame sends a single WebSocket frame (server→client, never masked).
func (c *WSConn) writeFrame(opcode byte, payload []byte) error {
	n := len(payload)
	buf := make([]byte, 0, 10+n)
	buf = append(buf, 0x80|opcode) // FIN=1

	switch {
	case n < 126:
		buf = append(buf, byte(n))
	case n < 65536:
		ext := make([]byte, 2)
		binary.BigEndian.PutUint16(ext, uint16(n))
		buf = append(buf, 126)
		buf = append(buf, ext...)
	default:
		ext := make([]byte, 8)
		binary.BigEndian.PutUint64(ext, uint64(n))
		buf = append(buf, 127)
		buf = append(buf, ext...)
	}
	buf = append(buf, payload...)

	c.wmu.Lock()
	defer c.wmu.Unlock()
	c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	_, err := c.conn.Write(buf)
	return err
}

func (c *WSConn) WriteText(data []byte) error  { return c.writeFrame(wsOpcodeText, data) }
func (c *WSConn) writeClose() error            { return c.writeFrame(wsOpcodeClose, []byte{0x03, 0xE8}) }
func (c *WSConn) writePong(payload []byte) error { return c.writeFrame(wsOpcodePong, payload) }
func (c *WSConn) close()                       { c.conn.Close() }

// UpgradeWebSocket performs the RFC 6455 handshake over a hijacked HTTP connection.
// Returns a WSConn ready for framed reads/writes.
func UpgradeWebSocket(w http.ResponseWriter, r *http.Request) (*WSConn, error) {
	if r.Header.Get("Upgrade") != "websocket" {
		return nil, fmt.Errorf("not a websocket upgrade request")
	}
	clientKey := r.Header.Get("Sec-WebSocket-Key")
	if clientKey == "" {
		return nil, fmt.Errorf("missing Sec-WebSocket-Key header")
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return nil, fmt.Errorf("response writer does not support hijacking")
	}
	conn, bufrw, err := hijacker.Hijack()
	if err != nil {
		return nil, fmt.Errorf("hijack: %w", err)
	}

	resp := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + wsAcceptKey(clientKey) + "\r\n\r\n"
	if _, err := bufrw.WriteString(resp); err != nil {
		conn.Close()
		return nil, fmt.Errorf("write handshake: %w", err)
	}
	if err := bufrw.Flush(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("flush handshake: %w", err)
	}

	return &WSConn{conn: conn, r: bufrw.Reader}, nil
}

// ─── Chat event protocol ─────────────────────────────────────────────────────

// ChatEvent is the JSON envelope for all chat WebSocket messages.
type ChatEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// EncodeChatEvent encodes a chat event envelope as JSON.
func EncodeChatEvent(eventType string, data any) ([]byte, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return json.Marshal(ChatEvent{Type: eventType, Data: raw})
}

// ─── Client ──────────────────────────────────────────────────────────────────

// ChatClient represents one connected WebSocket viewer.
type ChatClient struct {
	ws          *WSConn
	streamID    string
	userID      string // empty → anonymous/read-only
	displayName string
	send        chan []byte  // buffered outgoing frames
	done        chan struct{} // closed when the write pump exits
	closeOnce   sync.Once
}

const clientSendBuf = 64

// NewChatClient creates a new chat client ready to be added to the hub.
func NewChatClient(ws *WSConn, streamID, userID, displayName string) *ChatClient {
	return &ChatClient{
		ws:          ws,
		streamID:    streamID,
		userID:      userID,
		displayName: displayName,
		send:        make(chan []byte, clientSendBuf),
		done:        make(chan struct{}),
	}
}

// closeSend closes the send channel exactly once, even if called from multiple goroutines.
func (c *ChatClient) closeSend() {
	c.closeOnce.Do(func() { close(c.send) })
}

// writePump drains the send channel to the WebSocket connection.
func (c *ChatClient) writePump() {
	defer func() {
		c.ws.writeClose()
		c.ws.close()
		close(c.done)
	}()
	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				return
			}
			if err := c.ws.WriteText(msg); err != nil {
				return
			}
		}
	}
}

// enqueue delivers a message to the client; evicts the client if the buffer is full.
func (c *ChatClient) enqueue(msg []byte) {
	select {
	case c.send <- msg:
	default:
		// slow consumer — disconnect
		c.closeSend()
	}
}

// ─── Room ────────────────────────────────────────────────────────────────────

type chatRoom struct {
	streamID string
	clients  map[*ChatClient]struct{}
	mu       sync.RWMutex
}

func newChatRoom(streamID string) *chatRoom {
	return &chatRoom{
		streamID: streamID,
		clients:  make(map[*ChatClient]struct{}),
	}
}

func (rm *chatRoom) add(c *ChatClient) {
	rm.mu.Lock()
	rm.clients[c] = struct{}{}
	rm.mu.Unlock()
}

func (rm *chatRoom) remove(c *ChatClient) {
	rm.mu.Lock()
	delete(rm.clients, c)
	rm.mu.Unlock()
}

func (rm *chatRoom) size() int {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return len(rm.clients)
}

// broadcast delivers a message to every client in the room.
func (rm *chatRoom) broadcast(msg []byte) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	for c := range rm.clients {
		c.enqueue(msg)
	}
}

// sendToUser delivers a message only to the named user (for ban/timeout notifications).
func (rm *chatRoom) sendToUser(userID string, msg []byte) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	for c := range rm.clients {
		if c.userID == userID {
			c.enqueue(msg)
		}
	}
}

// ─── Hub ─────────────────────────────────────────────────────────────────────

// ChatHub manages all active chat rooms and the Redis pub/sub subscriptions.
type ChatHub struct {
	mu           sync.RWMutex
	rooms        map[string]*chatRoom          // streamID → room
	unsubs       map[string]context.CancelFunc // streamID → cancel fn for Redis sub goroutine
	redis        *redis.Client
	logger       *slog.Logger
	chatMsgsTotal atomic.Int64 // total user messages sent since process start
}

// NewChatHub creates a new hub. Call Run() to start the background worker.
func NewChatHub(redisClient *redis.Client, logger *slog.Logger) *ChatHub {
	return &ChatHub{
		rooms:  make(map[string]*chatRoom),
		unsubs: make(map[string]context.CancelFunc),
		redis:  redisClient,
		logger: logger,
	}
}

func chatChannel(streamID string) string { return "chat:stream:" + streamID }

// getOrCreateRoom returns the room for streamID, starting a Redis sub if new.
func (h *ChatHub) getOrCreateRoom(parentCtx context.Context, streamID string) *chatRoom {
	h.mu.Lock()
	defer h.mu.Unlock()

	if rm, ok := h.rooms[streamID]; ok {
		return rm
	}

	rm := newChatRoom(streamID)
	h.rooms[streamID] = rm

	// Subscribe to Redis for cross-instance broadcast
	subCtx, cancel := context.WithCancel(parentCtx)
	h.unsubs[streamID] = cancel
	go h.redisSubscribe(subCtx, streamID, rm)

	return rm
}

// dropRoom removes a room if it's empty (called after a client leaves).
func (h *ChatHub) dropRoom(streamID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	rm, ok := h.rooms[streamID]
	if !ok {
		return
	}
	if rm.size() > 0 {
		return
	}
	delete(h.rooms, streamID)
	if cancel, ok := h.unsubs[streamID]; ok {
		cancel()
		delete(h.unsubs, streamID)
	}
}

// Viewers returns the total number of WebSocket clients currently connected
// across all chat rooms on this instance.
func (h *ChatHub) Viewers() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	total := 0
	for _, rm := range h.rooms {
		total += rm.size()
	}
	return total
}

// RecordMessage increments the lifetime chat message counter.
// Call this once per successfully persisted and published user message.
func (h *ChatHub) RecordMessage() { h.chatMsgsTotal.Add(1) }

// ChatMsgsTotal returns the total number of chat messages sent since process start.
func (h *ChatHub) ChatMsgsTotal() int64 { return h.chatMsgsTotal.Load() }

// redisSubscribe listens on the stream's Redis pub/sub channel and broadcasts
// incoming payloads to all local clients in the room.
func (h *ChatHub) redisSubscribe(ctx context.Context, streamID string, rm *chatRoom) {
	pubsub := h.redis.Subscribe(ctx, chatChannel(streamID))
	defer pubsub.Close()

	for {
		msg, err := pubsub.ReceiveMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return // cancelled
			}
			h.logger.Error("chat redis subscribe", "stream_id", streamID, "error", err)
			return
		}
		rm.broadcast([]byte(msg.Payload))
	}
}

// Publish sends a chat event to the Redis channel so all API instances deliver it.
func (h *ChatHub) Publish(ctx context.Context, streamID string, payload []byte) error {
	return h.redis.Publish(ctx, chatChannel(streamID), string(payload)).Err()
}

// BroadcastToUser sends a message only to clients of that user in the stream (local only).
func (h *ChatHub) BroadcastToUser(streamID, userID string, payload []byte) {
	h.mu.RLock()
	rm, ok := h.rooms[streamID]
	h.mu.RUnlock()
	if ok {
		rm.sendToUser(userID, payload)
	}
}

// AddClient registers a client in the appropriate room and starts its I/O pumps.
// It blocks until the client disconnects.
func (h *ChatHub) AddClient(ctx context.Context, c *ChatClient, onMessage func(userID, body string) error) {
	rm := h.getOrCreateRoom(ctx, c.streamID)
	rm.add(c)

	go c.writePump()

	// Read pump (this goroutine)
	defer func() {
		rm.remove(c)
		c.closeSend() // safe even if enqueue already closed it
		h.dropRoom(c.streamID)
		<-c.done // wait for write pump to exit
	}()

	for {
		opcode, payload, err := c.ws.readFrame()
		if err != nil {
			return
		}

		switch opcode {
		case wsOpcodeClose:
			return

		case wsOpcodePing:
			if err := c.ws.writePong(payload); err != nil {
				return
			}

		case wsOpcodeText:
			if c.userID == "" {
				// anonymous — ignore sends
				continue
			}
			var msg struct {
				Type string `json:"type"`
				Body string `json:"body"`
			}
			if err := json.Unmarshal(payload, &msg); err != nil {
				continue
			}
			switch msg.Type {
			case "message":
				if err := onMessage(c.userID, msg.Body); err != nil {
					errBytes, _ := EncodeChatEvent("error", map[string]string{"message": err.Error()})
					c.enqueue(errBytes)
				}
			case "ping":
				pong, _ := EncodeChatEvent("pong", nil)
				c.enqueue(pong)
			}
		}
	}
}
