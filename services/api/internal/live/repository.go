package live

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles database operations for live streams.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new live stream repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Stream represents a live stream record.
type Stream struct {
	ID               string     `json:"id"`
	OwnerID          string     `json:"owner_id"`
	Title            string     `json:"title"`
	Description      string     `json:"description,omitempty"`
	StreamKeyHash    string     `json:"-"` // never exposed
	Status           string     `json:"status"`
	RTMPURL          string     `json:"rtmp_url,omitempty"`
	HLSUrl           string     `json:"hls_url,omitempty"`
	WebRTCURL        string     `json:"webrtc_url,omitempty"`
	DVR              bool       `json:"dvr_enabled"`
	DVRWindowSeconds int        `json:"dvr_window_seconds"`
	StartedAt        *time.Time `json:"started_at,omitempty"`
	EndedAt          *time.Time `json:"ended_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// Create inserts a new live stream and returns it.
func (r *Repository) Create(ctx context.Context, s *Stream) (*Stream, error) {
	row := r.db.QueryRow(ctx,
		`INSERT INTO live_streams
		 (owner_id, title, description, stream_key_hash, status, dvr_enabled, dvr_window_seconds)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, created_at, updated_at`,
		s.OwnerID, s.Title, s.Description, s.StreamKeyHash, s.Status, s.DVR, s.DVRWindowSeconds,
	)
	if err := row.Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return nil, err
	}
	return s, nil
}

// GetByID returns a single live stream by ID.
func (r *Repository) GetByID(ctx context.Context, id string) (*Stream, error) {
	s := &Stream{}
	err := r.db.QueryRow(ctx,
		`SELECT id, owner_id, title, COALESCE(description, ''), stream_key_hash,
		        status::text, rtmp_url, hls_url, webrtc_url, dvr_enabled, dvr_window_seconds,
		        started_at, ended_at, created_at, updated_at
		 FROM live_streams WHERE id = $1`,
		id,
	).Scan(&s.ID, &s.OwnerID, &s.Title, &s.Description, &s.StreamKeyHash,
		&s.Status, &s.RTMPURL, &s.HLSUrl, &s.WebRTCURL, &s.DVR, &s.DVRWindowSeconds,
		&s.StartedAt, &s.EndedAt, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return s, nil
}

// GetByStreamKeyHash looks up a stream by its (hashed) stream key.
// Used by the live helper to match RTMP pushes to streams.
func (r *Repository) GetByStreamKeyHash(ctx context.Context, hash string) (*Stream, error) {
	s := &Stream{}
	err := r.db.QueryRow(ctx,
		`SELECT id, owner_id, title, COALESCE(description, ''), stream_key_hash,
		        status::text, rtmp_url, hls_url, webrtc_url, dvr_enabled, dvr_window_seconds,
		        started_at, ended_at, created_at, updated_at
		 FROM live_streams WHERE stream_key_hash = $1`,
		hash,
	).Scan(&s.ID, &s.OwnerID, &s.Title, &s.Description, &s.StreamKeyHash,
		&s.Status, &s.RTMPURL, &s.HLSUrl, &s.WebRTCURL, &s.DVR, &s.DVRWindowSeconds,
		&s.StartedAt, &s.EndedAt, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return s, nil
}

// ListByOwner returns streams owned by a user, newest first.
func (r *Repository) ListByOwner(ctx context.Context, ownerID string, limit, offset int) ([]*Stream, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := r.db.Query(ctx,
		`SELECT id, owner_id, title, COALESCE(description, ''), stream_key_hash,
		        status::text, rtmp_url, hls_url, webrtc_url, dvr_enabled, dvr_window_seconds,
		        started_at, ended_at, created_at, updated_at
		 FROM live_streams
		 WHERE owner_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`,
		ownerID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var streams []*Stream
	for rows.Next() {
		s := &Stream{}
		if err := rows.Scan(&s.ID, &s.OwnerID, &s.Title, &s.Description, &s.StreamKeyHash,
			&s.Status, &s.RTMPURL, &s.HLSUrl, &s.WebRTCURL, &s.DVR, &s.DVRWindowSeconds,
			&s.StartedAt, &s.EndedAt, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		streams = append(streams, s)
	}
	if streams == nil {
		streams = []*Stream{}
	}
	return streams, nil
}

// Update updates mutable fields on a live stream.
func (r *Repository) Update(ctx context.Context, id string, ownerID string, title, description *string) (*Stream, error) {
	// Build dynamic SET clause
	setClauses := []string{"updated_at = NOW()"}
	args := []interface{}{}
	i := 1

	if title != nil {
		setClauses = append(setClauses, "title = $"+itoa(i))
		args = append(args, *title)
		i++
	}
	if description != nil {
		setClauses = append(setClauses, "description = $"+itoa(i))
		args = append(args, *description)
		i++
	}

	args = append(args, id, ownerID)
	query := "UPDATE live_streams SET " + joinStrings(setClauses, ", ") + " WHERE id = $" + itoa(i) + " AND owner_id = $" + itoa(i+1) + " RETURNING id, owner_id, title, COALESCE(description, ''), stream_key_hash, status::text, rtmp_url, hls_url, webrtc_url, dvr_enabled, dvr_window_seconds, started_at, ended_at, created_at, updated_at"

	s := &Stream{}
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&s.ID, &s.OwnerID, &s.Title, &s.Description, &s.StreamKeyHash,
		&s.Status, &s.RTMPURL, &s.HLSUrl, &s.WebRTCURL, &s.DVR, &s.DVRWindowSeconds,
		&s.StartedAt, &s.EndedAt, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return s, nil
}

// UpdateStatus changes the stream status and optionally sets started_at or ended_at.
func (r *Repository) UpdateStatus(ctx context.Context, id string, status string, startedAt, endedAt *time.Time) error {
	setClauses := []string{"updated_at = NOW()"}
	args := []interface{}{}
	i := 1

	setClauses = append(setClauses, "status = $"+itoa(i))
	args = append(args, status)
	i++

	if startedAt != nil {
		setClauses = append(setClauses, "started_at = $"+itoa(i))
		args = append(args, *startedAt)
		i++
	}
	if endedAt != nil {
		setClauses = append(setClauses, "ended_at = $"+itoa(i))
		args = append(args, *endedAt)
		i++
	}

	args = append(args, id)
	query := "UPDATE live_streams SET " + joinStrings(setClauses, ", ") + " WHERE id = $" + itoa(i)

	_, err := r.db.Exec(ctx, query, args...)
	return err
}

// SetStreamURLs updates the RTMP, HLS, and WebRTC URLs for a stream.
func (r *Repository) SetStreamURLs(ctx context.Context, id string, rtmpURL, hlsURL, webrtcURL string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE live_streams SET rtmp_url = $1, hls_url = $2, webrtc_url = $3, updated_at = NOW() WHERE id = $4`,
		rtmpURL, hlsURL, webrtcURL, id,
	)
	return err
}

// Delete soft-deletes (or hard-deletes) a live stream by ID, ensuring owner match.
func (r *Repository) Delete(ctx context.Context, id string, ownerID string) (bool, error) {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM live_streams WHERE id = $1 AND owner_id = $2`,
		id, ownerID,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// ListLive returns all streams currently in 'live' status.
func (r *Repository) ListLive(ctx context.Context) ([]*Stream, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, owner_id, title, COALESCE(description, ''), stream_key_hash,
		        status::text, rtmp_url, hls_url, webrtc_url, dvr_enabled, dvr_window_seconds,
		        started_at, ended_at, created_at, updated_at
		 FROM live_streams
		 WHERE status = 'live'
		 ORDER BY started_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var streams []*Stream
	for rows.Next() {
		s := &Stream{}
		if err := rows.Scan(&s.ID, &s.OwnerID, &s.Title, &s.Description, &s.StreamKeyHash,
			&s.Status, &s.RTMPURL, &s.HLSUrl, &s.WebRTCURL, &s.DVR, &s.DVRWindowSeconds,
			&s.StartedAt, &s.EndedAt, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		streams = append(streams, s)
	}
	if streams == nil {
		streams = []*Stream{}
	}
	return streams, nil
}

// ListEndedWithDVR returns ended streams that have DVR enabled and have not yet
// been cleaned (dvr_cleaned_at IS NULL). Used by DVRCleaner.
func (r *Repository) ListEndedWithDVR(ctx context.Context) ([]*Stream, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, owner_id, title, COALESCE(description, ''), stream_key_hash,
		        status::text, rtmp_url, hls_url, webrtc_url, dvr_enabled, dvr_window_seconds,
		        started_at, ended_at, created_at, updated_at
		 FROM live_streams
		 WHERE status = 'ended' AND dvr_enabled = true AND dvr_cleaned_at IS NULL`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var streams []*Stream
	for rows.Next() {
		s := &Stream{}
		if err := rows.Scan(&s.ID, &s.OwnerID, &s.Title, &s.Description, &s.StreamKeyHash,
			&s.Status, &s.RTMPURL, &s.HLSUrl, &s.WebRTCURL, &s.DVR, &s.DVRWindowSeconds,
			&s.StartedAt, &s.EndedAt, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		streams = append(streams, s)
	}
	if streams == nil {
		streams = []*Stream{}
	}
	return streams, nil
}

// MarkDVRCleaned stamps dvr_cleaned_at so the cleaner skips this stream next sweep.
func (r *Repository) MarkDVRCleaned(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE live_streams SET dvr_cleaned_at = NOW(), updated_at = NOW() WHERE id = $1`,
		id,
	)
	return err
}

// GetActiveByOwner returns the single active (idle or live) stream for an owner.
func (r *Repository) GetActiveByOwner(ctx context.Context, ownerID string) (*Stream, error) {
	s := &Stream{}
	err := r.db.QueryRow(ctx,
		`SELECT id, owner_id, title, COALESCE(description, ''), stream_key_hash,
		        status::text, rtmp_url, hls_url, webrtc_url, dvr_enabled, dvr_window_seconds,
		        started_at, ended_at, created_at, updated_at
		 FROM live_streams
		 WHERE owner_id = $1 AND status IN ('idle', 'live')
		 ORDER BY created_at DESC
		 LIMIT 1`,
		ownerID,
	).Scan(&s.ID, &s.OwnerID, &s.Title, &s.Description, &s.StreamKeyHash,
		&s.Status, &s.RTMPURL, &s.HLSUrl, &s.WebRTCURL, &s.DVR, &s.DVRWindowSeconds,
		&s.StartedAt, &s.EndedAt, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return s, nil
}

// itoa converts an int to its string representation (avoiding strconv import).
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	s := ""
	for i > 0 {
		s = string(rune('0'+i%10)) + s
		i /= 10
	}
	return s
}

// joinStrings joins strings with a separator.
func joinStrings(elems []string, sep string) string {
	if len(elems) == 0 {
		return ""
	}
	result := elems[0]
	for _, e := range elems[1:] {
		result += sep + e
	}
	return result
}