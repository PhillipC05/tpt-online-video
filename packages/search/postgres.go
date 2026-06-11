package search

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresProvider struct {
	db *pgxpool.Pool
}

func NewPostgresProvider(db *pgxpool.Pool) *PostgresProvider {
	return &PostgresProvider{db: db}
}

func (p *PostgresProvider) Search(ctx context.Context, query Query) (Result, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	offset := query.Offset
	if offset < 0 {
		offset = 0
	}

	searchText := strings.TrimSpace(query.Text)
	if searchText == "" {
		return Result{Items: []Video{}}, nil
	}

	rows, err := p.db.Query(ctx, `
		SELECT
			video_id,
			title,
			description,
			owner_display_name,
			tags,
			ts_rank(search_vector, websearch_to_tsquery('english', $1)) AS score,
			indexed_at
		FROM search_documents
		WHERE search_vector @@ websearch_to_tsquery('english', $1)
		ORDER BY score DESC, indexed_at DESC
		LIMIT $2 OFFSET $3
	`, searchText, limit, offset)
	if err != nil {
		return Result{}, fmt.Errorf("search videos: %w", err)
	}
	defer rows.Close()

	items := make([]Video, 0, limit)
	for rows.Next() {
		var item Video
		if err := rows.Scan(&item.ID, &item.Title, &item.Description, &item.OwnerDisplayName, &item.Tags, &item.Score, &item.IndexedAt); err != nil {
			return Result{}, fmt.Errorf("scan search result: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return Result{}, fmt.Errorf("iterate search results: %w", err)
	}

	var total int
	if err := p.db.QueryRow(ctx, `
		SELECT count(*)
		FROM search_documents
		WHERE search_vector @@ websearch_to_tsquery('english', $1)
	`, searchText).Scan(&total); err != nil {
		return Result{}, fmt.Errorf("count search results: %w", err)
	}

	return Result{Items: items, Total: total}, nil
}

func (p *PostgresProvider) IndexVideo(ctx context.Context, video Video) error {
	if video.ID == "" || video.Title == "" {
		return fmt.Errorf("video id and title are required")
	}
	_, err := p.db.Exec(ctx, `
		INSERT INTO search_documents (video_id, title, description, owner_display_name, tags)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (video_id) DO UPDATE SET
			title = EXCLUDED.title,
			description = EXCLUDED.description,
			owner_display_name = EXCLUDED.owner_display_name,
			tags = EXCLUDED.tags
	`, video.ID, video.Title, video.Description, video.OwnerDisplayName, video.Tags)
	if err != nil {
		return fmt.Errorf("index video: %w", err)
	}
	return nil
}

func (p *PostgresProvider) DeleteVideo(ctx context.Context, videoID string) error {
	_, err := p.db.Exec(ctx, `DELETE FROM search_documents WHERE video_id = $1`, videoID)
	if err != nil {
		return fmt.Errorf("delete video search document: %w", err)
	}
	return nil
}

func (p *PostgresProvider) Health(ctx context.Context) error {
	return p.db.Ping(ctx)
}