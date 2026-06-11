package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/tpt-online-video/packages/media"
	"github.com/tpt-online-video/packages/moderation"
	"github.com/tpt-online-video/packages/storage"
)

// testUploadHandler creates an UploadHandler with a local storage and no-op scanner for testing.
func testUploadHandler(t *testing.T) *UploadHandler {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	db, err := pgxpool.New(context.Background(), "postgres://tpt:tpt@localhost:5432/tpt?sslmode=disable")
	if err != nil {
		t.Skip("postgres not available:", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skip("redis not available:", err)
	}

	store, err := storage.NewLocal("testdata/uploads")
	if err != nil {
		t.Fatalf("create local storage: %v", err)
	}

	queue := media.NewQueue(rdb, "transcode:test", "test-workers", "test")

	return &UploadHandler{
		logger:   logger,
		db:       db,
		redis:    rdb,
		storage:  store,
		queue:    queue,
		baseURL:  "http://localhost:8080",
		scanner:  moderation.NewNopScanner(),
		maxBytes: 100 * 1024 * 1024, // 100MB for tests
	}
}

// withTestUserContext injects a user ID into the request context.
func withTestUserContext(r *http.Request, userID string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), "user_id", userID))
}

// setupChiContext adds chi URL params to the request context.
func setupChiContext(r *http.Request, params map[string]string) *http.Request {
	chiCtx := chi.NewRouteContext()
	for k, v := range params {
		chiCtx.URLParams.Add(k, v)
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, chiCtx))
}

func TestCreateSession(t *testing.T) {
	h := testUploadHandler(t)

	tests := []struct {
		name       string
		body       map[string]interface{}
		wantStatus int
	}{
		{
			name: "valid mp4 upload",
			body: map[string]interface{}{
				"filename":  "test.mp4",
				"mime_type": "video/mp4",
				"byte_size": 1048576,
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "valid webm upload",
			body: map[string]interface{}{
				"filename":  "test.webm",
				"mime_type": "video/webm",
				"byte_size": 2097152,
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "empty filename",
			body: map[string]interface{}{
				"filename":  "",
				"mime_type": "video/mp4",
				"byte_size": 1024,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "invalid file extension",
			body: map[string]interface{}{
				"filename":  "test.exe",
				"mime_type": "application/x-msdownload",
				"byte_size": 1024,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "mime type mismatch",
			body: map[string]interface{}{
				"filename":  "test.mp4",
				"mime_type": "video/webm",
				"byte_size": 1024,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "file too large",
			body: map[string]interface{}{
				"filename":  "test.mp4",
				"mime_type": "video/mp4",
				"byte_size": 200 * 1024 * 1024, // 200MB exceeds 100MB limit
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "zero byte size",
			body: map[string]interface{}{
				"filename":  "test.mp4",
				"mime_type": "video/mp4",
				"byte_size": 0,
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/upload", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = withTestUserContext(req, "test-user-id")

			w := httptest.NewRecorder()
			h.CreateSession(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("CreateSession() status = %d, want %d; body=%s", w.Code, tt.wantStatus, w.Body.String())
			}

			if tt.wantStatus == http.StatusCreated {
				var resp CreateUploadResponse
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("unmarshal response: %v", err)
				}
				if resp.SessionID == "" {
					t.Error("expected non-empty session_id")
				}
				if resp.ExpiresAt == "" {
					t.Error("expected non-empty expires_at")
				}
				if resp.MaxBytes <= 0 {
					t.Error("expected positive max_bytes")
				}
			}
		})
	}
}

func TestCancelUpload(t *testing.T) {
	h := testUploadHandler(t)

	// Create a session first
	sessionID := createTestSession(t, h)
	if sessionID == "" {
		t.Fatal("failed to create test session")
	}

	// Cancel the session
	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload/"+sessionID+"/cancel", nil)
	req = withTestUserContext(req, "test-user-id")
	req = setupChiContext(req, map[string]string{"sessionID": sessionID})

	w := httptest.NewRecorder()
	h.CancelUpload(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("CancelUpload() status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp["status"] != "cancelled" {
		t.Errorf("expected status 'cancelled', got %q", resp["status"])
	}

	// Verify cancellation is idempotent
	w2 := httptest.NewRecorder()
	h.CancelUpload(w2, req)
	if w2.Code != http.StatusBadRequest {
		t.Errorf("second CancelUpload() status = %d, want %d", w2.Code, http.StatusBadRequest)
	}
}

func TestGetUploadStatus(t *testing.T) {
	h := testUploadHandler(t)

	// Create a session
	sessionID := createTestSession(t, h)
	if sessionID == "" {
		t.Fatal("failed to create test session")
	}

	// Get status
	req := httptest.NewRequest(http.MethodGet, "/api/v1/upload/"+sessionID, nil)
	req = withTestUserContext(req, "test-user-id")
	req = setupChiContext(req, map[string]string{"sessionID": sessionID})

	w := httptest.NewRecorder()
	h.GetUploadStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GetUploadStatus() status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	var status UploadSessionStatus
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if status.SessionID != sessionID {
		t.Errorf("expected session_id %q, got %q", sessionID, status.SessionID)
	}
	if status.Status != "pending" {
		t.Errorf("expected status 'pending', got %q", status.Status)
	}
}

func TestListUploadSessions(t *testing.T) {
	h := testUploadHandler(t)

	// Create a couple sessions
	createTestSession(t, h)
	createTestSession(t, h)

	// List sessions
	req := httptest.NewRequest(http.MethodGet, "/api/v1/upload/sessions", nil)
	req = withTestUserContext(req, "test-user-id")

	w := httptest.NewRecorder()
	h.ListUploadSessions(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("ListUploadSessions() status = %d, want %d", w.Code, http.StatusOK)
	}

	var sessions []UploadSessionStatus
	if err := json.Unmarshal(w.Body.Bytes(), &sessions); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(sessions) < 2 {
		t.Errorf("expected at least 2 sessions, got %d", len(sessions))
	}
}

func TestUnauthenticatedRequests(t *testing.T) {
	h := testUploadHandler(t)

	endpoints := []struct {
		method string
		path   string
		handler func(http.ResponseWriter, *http.Request)
	}{
		{http.MethodPost, "/api/v1/upload", h.CreateSession},
		{http.MethodPost, "/api/v1/upload/session-id/cancel", h.CancelUpload},
		{http.MethodGet, "/api/v1/upload/session-id", h.GetUploadStatus},
		{http.MethodGet, "/api/v1/upload/sessions", h.ListUploadSessions},
		{http.MethodPost, "/api/v1/upload/session-id/complete", h.CompleteUpload},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, nil)
			if ep.path != "/api/v1/upload" && ep.path != "/api/v1/upload/sessions" {
				req = setupChiContext(req, map[string]string{"sessionID": "session-id"})
			}

			// Create a valid body for create session
			if ep.path == "/api/v1/upload" {
				body, _ := json.Marshal(CreateUploadRequest{
					Filename: "test.mp4",
					MimeType: "video/mp4",
					ByteSize: 1024,
				})
				req.Body = bytes.NewReader(body)
				req.Header.Set("Content-Type", "application/json")
			}

			w := httptest.NewRecorder()
			ep.handler(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Errorf("expected 401 Unauthorized for %s %s, got %d", ep.method, ep.path, w.Code)
			}
		})
	}
}

// createTestSession is a helper that creates an upload session and returns its ID.
func createTestSession(t *testing.T, h *UploadHandler) string {
	t.Helper()

	body, _ := json.Marshal(CreateUploadRequest{
		Filename: "test.mp4",
		MimeType: "video/mp4",
		ByteSize: 1048576,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withTestUserContext(req, "test-user-id")

	w := httptest.NewRecorder()
	h.CreateSession(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("createTestSession: status=%d body=%s", w.Code, w.Body.String())
	}

	var resp CreateUploadResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("createTestSession: unmarshal error: %v", err)
	}

	return resp.SessionID
}

func TestValidateFileType(t *testing.T) {
	tests := []struct {
		name      string
		filename  string
		mimeType  string
		wantOK    bool
	}{
		{".mp4 with video/mp4", "video.mp4", "video/mp4", true},
		{".webm with video/webm", "video.webm", "video/webm", true},
		{".mov with video/quicktime", "video.mov", "video/quicktime", true},
		{".exe with application/x-msdownload", "virus.exe", "application/x-msdownload", false},
		{".mp4 wrong mime", "video.mp4", "video/webm", false},
		{"no extension", "README", "video/mp4", false},
		{"empty filename", "", "video/mp4", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := moderation.ValidateFileType(tt.filename, tt.mimeType)
			if tt.wantOK && err != nil {
				t.Errorf("ValidateFileType(%q, %q): unexpected error: %v", tt.filename, tt.mimeType, err)
			}
			if !tt.wantOK && err == nil {
				t.Errorf("ValidateFileType(%q, %q): expected error, got nil", tt.filename, tt.mimeType)
			}
		})
	}
}

func TestValidateFileSize(t *testing.T) {
	tests := []struct {
		name     string
		size     int64
		maxBytes int64
		wantOK   bool
	}{
		{"valid size", 1024, 10 * 1024 * 1024, true},
		{"exact max", 10 * 1024 * 1024, 10 * 1024 * 1024, true},
		{"over max", 11 * 1024 * 1024, 10 * 1024 * 1024, false},
		{"zero size", 0, 10 * 1024 * 1024, false},
		{"negative size", -1, 10 * 1024 * 1024, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := moderation.ValidateFileSize(tt.size, tt.maxBytes)
			if tt.wantOK && err != nil {
				t.Errorf("ValidateFileSize(%d, %d): unexpected error: %v", tt.size, tt.maxBytes, err)
			}
			if !tt.wantOK && err == nil {
				t.Errorf("ValidateFileSize(%d, %d): expected error, got nil", tt.size, tt.maxBytes)
			}
		})
	}
}

func TestNopScanner(t *testing.T) {
	scanner := moderation.NewNopScanner()

	if scanner.Name() != "noop" {
		t.Errorf("expected name 'noop', got %q", scanner.Name())
	}

	result, err := scanner.Scan(context.Background(), "test.mp4", bytes.NewReader([]byte("test content")))
	if err != nil {
		t.Errorf("Scan() unexpected error: %v", err)
	}
	if result.Infected {
		t.Error("expected no infection from nop scanner")
	}
	if result.Threat != "" {
		t.Errorf("expected empty threat, got %q", result.Threat)
	}
}

func TestUploadChunk(t *testing.T) {
	h := testUploadHandler(t)
	sessionID := createTestSession(t, h)

	chunkData := bytes.Repeat([]byte("a"), 1024)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload/"+sessionID+"/chunk", bytes.NewReader(chunkData))
	req.Header.Set("Content-Type", "application/octet-stream")
	req = withTestUserContext(req, "test-user-id")
	req = setupChiContext(req, map[string]string{"sessionID": sessionID})

	w := httptest.NewRecorder()
	h.UploadChunk(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("UploadChunk() status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp ChunkUploadResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.ReceivedBytes != int64(len(chunkData)) {
		t.Errorf("expected received_bytes=%d, got %d", len(chunkData), resp.ReceivedBytes)
	}
	if resp.Status != "uploading" {
		t.Errorf("expected status 'uploading', got %q", resp.Status)
	}
}

func TestCompleteUpload(t *testing.T) {
	h := testUploadHandler(t)
	sessionID := createTestSession(t, h)

	// Upload a small chunk to put session in uploading state
	chunkData := bytes.Repeat([]byte("b"), 512)
	chunkReq := httptest.NewRequest(http.MethodPost, "/api/v1/upload/"+sessionID+"/chunk", bytes.NewReader(chunkData))
	chunkReq.Header.Set("Content-Type", "application/octet-stream")
	chunkReq = withTestUserContext(chunkReq, "test-user-id")
	chunkReq = setupChiContext(chunkReq, map[string]string{"sessionID": sessionID})
	h.UploadChunk(httptest.NewRecorder(), chunkReq)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload/"+sessionID+"/complete", nil)
	req = withTestUserContext(req, "test-user-id")
	req = setupChiContext(req, map[string]string{"sessionID": sessionID})

	w := httptest.NewRecorder()
	h.CompleteUpload(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("CompleteUpload() status = %d, want %d; body=%s", w.Code, http.StatusCreated, w.Body.String())
	}

	var resp CompleteUploadResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.VideoID == "" {
		t.Error("expected non-empty video_id")
	}
	if resp.Status != "queued" {
		t.Errorf("expected status 'queued', got %q", resp.Status)
	}
}

// infectedScanner is a test scanner that always reports the content as infected.
type infectedScanner struct{}

func (s *infectedScanner) Scan(_ context.Context, _ string, _ io.Reader) (*moderation.ScanResult, error) {
	return &moderation.ScanResult{Infected: true, Threat: "Test.Malware.Generic"}, nil
}
func (s *infectedScanner) Name() string { return "infected-test" }

func TestScannerRejectsInfectedChunk(t *testing.T) {
	h := testUploadHandler(t)
	h.scanner = &infectedScanner{}

	sessionID := createTestSession(t, h)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload/"+sessionID+"/chunk", bytes.NewReader([]byte("malicious content")))
	req.Header.Set("Content-Type", "application/octet-stream")
	req = withTestUserContext(req, "test-user-id")
	req = setupChiContext(req, map[string]string{"sessionID": sessionID})

	w := httptest.NewRecorder()
	h.UploadChunk(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("UploadChunk() with infected file status = %d, want %d; body=%s", w.Code, http.StatusUnprocessableEntity, w.Body.String())
	}
}

// Ensure tests compile by referencing time
var _ = time.Now