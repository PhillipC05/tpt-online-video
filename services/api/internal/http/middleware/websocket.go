package middleware

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// WebSocketConfig configures the WebSocket middleware.
type WebSocketConfig struct {
	// MaxConnections is the maximum number of concurrent WebSocket connections per user/IP.
	MaxConnections int
	// HandshakeTimeout is the maximum time allowed for the WebSocket handshake.
	HandshakeTimeout time.Duration
	// ReadBufferSize is the buffer size for reading messages.
	ReadBufferSize int
	// WriteBufferSize is the buffer size for writing messages.
	WriteBufferSize int
	// AllowedOrigins are the origins allowed to establish WebSocket connections.
	AllowedOrigins []string
}

// DefaultWebSocketConfig returns sensible defaults for WebSocket configuration.
func DefaultWebSocketConfig() WebSocketConfig {
	return WebSocketConfig{
		MaxConnections:   10,
		HandshakeTimeout: 10 * time.Second,
		ReadBufferSize:   4096,
		WriteBufferSize:  4096,
		AllowedOrigins:   []string{"*"},
	}
}

// WebSocketMiddleware provides WebSocket connection management and security.
type WebSocketMiddleware struct {
	config WebSocketConfig
	logger *slog.Logger
	redis  *redis.Client

	// In-memory connection tracking for simplicity.
	// In production, this should use Redis for distributed tracking.
	mu          sync.RWMutex
	connections map[string]int // key (userID or IP) -> connection count
}

// NewWebSocketMiddleware creates a new WebSocket middleware.
func NewWebSocketMiddleware(logger *slog.Logger, redis *redis.Client, config WebSocketConfig) *WebSocketMiddleware {
	if config.MaxConnections == 0 {
		config = DefaultWebSocketConfig()
	}
	return &WebSocketMiddleware{
		config:      config,
		logger:      logger,
		redis:       redis,
		connections: make(map[string]int),
	}
}

// OriginCheck is a middleware that validates the Origin header for WebSocket upgrades.
func (wm *WebSocketMiddleware) OriginCheck(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only check WebSocket upgrade requests
		if r.Header.Get("Upgrade") != "websocket" {
			next.ServeHTTP(w, r)
			return
		}

		origin := r.Header.Get("Origin")
		if origin == "" {
			// No origin header, allow if wildcard is set
			for _, allowed := range wm.config.AllowedOrigins {
				if allowed == "*" {
					next.ServeHTTP(w, r)
					return
				}
			}
			WriteForbidden(w, "origin header required for WebSocket connections")
			return
		}

		// Check allowed origins
		allowed := false
		for _, allowedOrigin := range wm.config.AllowedOrigins {
			if allowedOrigin == "*" || allowedOrigin == origin {
				allowed = true
				break
			}
		}

		if !allowed {
			wm.logger.Warn("websocket origin not allowed",
				"origin", origin,
				"path", r.URL.Path,
			)
			WriteForbidden(w, "origin not allowed")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ConnectionLimit is a middleware that limits concurrent WebSocket connections per user/IP.
func (wm *WebSocketMiddleware) ConnectionLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only check WebSocket upgrade requests
		if r.Header.Get("Upgrade") != "websocket" {
			next.ServeHTTP(w, r)
			return
		}

		// Determine the key for connection tracking
		key := wm.connectionKey(r)

		wm.mu.Lock()
		count := wm.connections[key]
		if count >= wm.config.MaxConnections {
			wm.mu.Unlock()
			wm.logger.Warn("websocket connection limit exceeded",
				"key", key,
				"count", count,
				"max", wm.config.MaxConnections,
				"path", r.URL.Path,
			)
			WriteTooManyRequests(w, fmt.Sprintf("maximum %d concurrent WebSocket connections allowed", wm.config.MaxConnections))
			return
		}
		wm.connections[key] = count + 1
		wm.mu.Unlock()

		// Wrap the response writer to track connection close
		cnl := &connectionListener{
			ResponseWriter: w,
			onClose: func() {
				wm.mu.Lock()
				wm.connections[key]--
				if wm.connections[key] <= 0 {
					delete(wm.connections, key)
				}
				wm.mu.Unlock()
			},
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), "ws_on_close", cnl.onClose)))
	})
}

// connectionKey returns the key used for connection tracking.
func (wm *WebSocketMiddleware) connectionKey(r *http.Request) string {
	// First try user ID
	if userID := GetUserID(r); userID != "" {
		return "user:" + userID
	}
	// Fall back to IP
	return "ip:" + IPKeyFunc(r)
}

// connectionListener wraps http.ResponseWriter to detect connection close.
type connectionListener struct {
	http.ResponseWriter
	onClose func()
	once    sync.Once
}

func (cl *connectionListener) CloseNotify() <-chan bool {
	return nil
}

// Hijack allows the WebSocket library to take over the connection.
func (cl *connectionListener) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := cl.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer does not support hijacking")
	}
	conn, bufrw, err := hijacker.Hijack()
	if err != nil {
		return nil, nil, err
	}
	// Track cleanup on connection close
	go func() {
		buf := make([]byte, 1)
		conn.SetReadDeadline(time.Now().Add(1 * time.Hour))
		for {
			_, err := conn.Read(buf)
			if err != nil {
				cl.once.Do(cl.onClose)
				return
			}
			conn.SetReadDeadline(time.Now().Add(1 * time.Hour))
		}
	}()
	return conn, bufrw, nil
}

// RedisPubSubManager provides Redis-backed WebSocket pub/sub for broadcasting messages
// across multiple API instances.
type RedisPubSubManager struct {
	redis    *redis.Client
	logger   *slog.Logger
	handlers map[string]func(string, string) // channel -> handler(message, senderID)
	mu       sync.RWMutex
}

// NewRedisPubSubManager creates a new Redis pub/sub manager.
func NewRedisPubSubManager(redis *redis.Client, logger *slog.Logger) *RedisPubSubManager {
	return &RedisPubSubManager{
		redis:    redis,
		logger:   logger,
		handlers: make(map[string]func(string, string)),
	}
}

// Subscribe subscribes to a Redis pub/sub channel and calls the handler when messages arrive.
func (m *RedisPubSubManager) Subscribe(ctx context.Context, channel string, handler func(message, senderID string)) error {
	m.mu.Lock()
	m.handlers[channel] = handler
	m.mu.Unlock()

	pubsub := m.redis.Subscribe(ctx, channel)
	go func() {
		defer pubsub.Close()
		for {
			msg, err := pubsub.ReceiveMessage(ctx)
			if err != nil {
				m.logger.Error("redis pubsub receive", "error", err, "channel", channel)
				return
			}
			m.mu.RLock()
			h, ok := m.handlers[channel]
			m.mu.RUnlock()
			if ok {
				// Message payload format: "senderID|message"
				payload := msg.Payload
				parts := splitN(payload, "|", 2)
				var senderID, message string
				if len(parts) == 2 {
					senderID = parts[0]
					message = parts[1]
				} else {
					message = payload
				}
				h(message, senderID)
			}
		}
	}()
	return nil
}

// Publish publishes a message to a Redis pub/sub channel.
func (m *RedisPubSubManager) Publish(ctx context.Context, channel, message, senderID string) error {
	payload := senderID + "|" + message
	return m.redis.Publish(ctx, channel, payload).Err()
}

// Unsubscribe removes a handler for a channel.
func (m *RedisPubSubManager) Unsubscribe(channel string) {
	m.mu.Lock()
	delete(m.handlers, channel)
	m.mu.Unlock()
}

func splitN(s, sep string, n int) []string {
	// Simple splitN implementation to avoid importing strings in this context
	result := make([]string, 0, n)
	for i := 0; i < n-1; i++ {
		idx := indexOf(s, sep)
		if idx < 0 {
			break
		}
		result = append(result, s[:idx])
		s = s[idx+len(sep):]
	}
	result = append(result, s)
	return result
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}