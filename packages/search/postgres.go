package search

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresProvider struct {
	db *pgxpool.Pool
}

func NewPostgresProvider(db *pgxpool.Pool) *PostgresProvider {
	return &PostgresProvider{db: db}
}

func (p *PostgresProvider) Autocomplete(ctx context.Context, prefix string, limit int) ([]string, error) {
	if prefix == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 20 {
		limit = 10
	}

	rows, err := p.db.Query(ctx, `
		SELECT DISTINCT title
		FROM search_documents
		WHERE title ILIKE $1
		ORDER BY title
		LIMIT $2
	`, prefix+"%", limit)
	if err != nil {
		return nil, fmt.Errorf("autocomplete: %w", err)
	}
	defer rows.Close()

	var suggestions []string
	for rows.Next() {
		var title string
		if err := rows.Scan(&title); err != nil {
			return nil, fmt.Errorf("scan autocomplete: %w", err)
		}
		suggestions = append(suggestions, title)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate autocomplete: %w", err)
	}
	return suggestions, nil
}

func (p *PostgresProvider) Search(ctx context.Context, query Query) (Result, error) {
	q := query.Normalized()
	searchText := strings.TrimSpace(q.Text)

	clauses := []string{"$1 = '' OR sd.search_vector @@ websearch_to_tsquery('english', $1)"}
	args := []any{searchText}

	if q.Duration != DurationAny {
		minSeconds, maxSeconds := q.Duration.bounds()
		if minSeconds > 0 {
			clauses = append(clauses, fmt.Sprintf("v.duration_seconds >= $%d", len(args)+1))
			args = append(args, minSeconds)
		}
		if maxSeconds > 0 {
			clauses = append(clauses, fmt.Sprintf("v.duration_seconds <= $%d", len(args)+1))
			args = append(args, maxSeconds)
		}
	}

	if q.UploadDate != UploadDateAny {
		cutoff, ok := q.UploadDate.cutoff()
		if !ok {
			return Result{}, fmt.Errorf("unknown upload date filter %q", q.UploadDate)
		}
		clauses = append(clauses, fmt.Sprintf("v.published_at >= $%d", len(args)+1))
		args = append(args, cutoff)
	}

	if q.MediaType != "" {
		clauses = append(clauses, fmt.Sprintf("sd.media_type = $%d", len(args)+1))
		args = append(args, string(q.MediaType))
	}

	if q.OwnerID != "" {
		clauses = append(clauses, fmt.Sprintf("v.owner_id = $%d", len(args)+1))
		args = append(args, q.OwnerID)
	}

	clauses = append(clauses,
		"v.visibility = 'public'",
		"v.status = 'ready'",
		"v.deleted_at IS NULL",
	)

	whereSQL := strings.Join(clauses, " AND\n")
	tsRank := "COALESCE(ts_rank_cd(sd.search_vector, websearch_to_tsquery('english', $1)), 0)"
	recencyScore := `CASE WHEN v.published_at IS NULL THEN 0 ELSE exp(-EXTRACT(EPOCH FROM (now() - v.published_at)) / 604800.0 / 4.0) END`
	viewScore := `ln(1 + v.view_count) / ln(1000001.0)`
	engagementScore := `(0.7 * (ln(1 + v.view_count) / ln(1000001.0)) + 0.3 * (ln(1 + v.like_count) / ln(10001.0)))`

	selectSQL := fmt.Sprintf(`
		WITH scored AS (
			SELECT
				sd.video_id,
				sd.title,
				sd.description,
				sd.owner_display_name,
				sd.tags,
				sd.media_type,
				v.duration_seconds,
				v.view_count,
				v.like_count,
				v.created_at,
				v.published_at,
				v.owner_id,
				sd.indexed_at,
				%s AS text_score,
				%s AS recency_score,
				%s AS view_score,
				%s AS engagement_score
			FROM search_documents sd
			JOIN videos v ON v.id = sd.video_id
			JOIN users u ON u.id = v.owner_id
			WHERE %s
		)
		SELECT
			video_id,
			title,
			coalesce(description, ''),
			owner_id,
			owner_display_name,
			tags,
			media_type,
			duration_seconds,
			view_count,
			like_count,
			created_at,
			published_at,
			indexed_at,
			text_score,
			recency_score,
			view_score,
			engagement_score,
			%s AS score
		FROM scored
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`,
		tsRank,
		recencyScore,
		viewScore,
		engagementScore,
		whereSQL,
		q.rankExpression(tsRank, recencyScore, viewScore, engagementScore),
		len(args)+1,
		len(args)+2,
	)

	args = append(args, q.Limit, q.Offset)

	rows, err := p.db.Query(ctx, selectSQL, args...)
	if err != nil {
		return Result{}, fmt.Errorf("search videos: %w", err)
	}
	defer rows.Close()

	items := make([]ResultItem, 0, q.Limit)
	for rows.Next() {
		var item ResultItem
		var publishedAt *time.Time
		if err := rows.Scan(
			&item.ID,
			&item.Title,
			&item.Description,
			&item.OwnerID,
			&item.OwnerDisplayName,
			&item.Tags,
			&item.MediaType,
			&item.DurationSeconds,
			&item.ViewCount,
			&item.LikeCount,
			&item.CreatedAt,
			&publishedAt,
			&item.IndexedAt,
			&item.TextScore,
			&item.RecencyScore,
			&item.ViewScore,
			&item.EngagementScore,
			&item.Score,
		); err != nil {
			return Result{}, fmt.Errorf("scan search result: %w", err)
		}
		item.PublishedAt = publishedAt
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return Result{}, fmt.Errorf("iterate search results: %w", err)
	}

	countSQL := fmt.Sprintf(`
		SELECT count(*)
		FROM search_documents sd
		JOIN videos v ON v.id = sd.video_id
		WHERE %s
	`, whereSQL)

	var total int
	if err := p.db.QueryRow(ctx, countSQL, args[:len(args)-2]...).Scan(&total); err != nil {
		return Result{}, fmt.Errorf("count search results: %w", err)
	}

	return Result{Items: items, Total: total, Query: q}, nil
}

func (p *PostgresProvider) IndexVideo(ctx context.Context, video Video) error {
	if video.ID == "" || video.Title == "" {
		return fmt.Errorf("video id and title are required")
	}

	mediaType := video.MediaType
	if mediaType == "" {
		mediaType = MediaTypeVOD
	}

	_, err := p.db.Exec(ctx, `
		INSERT INTO search_documents (
			video_id,
			title,
			description,
			owner_id,
			owner_display_name,
			tags,
			media_type,
			duration_seconds
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (video_id) DO UPDATE SET
			title = EXCLUDED.title,
			description = EXCLUDED.description,
			owner_id = EXCLUDED.owner_id,
			owner_display_name = EXCLUDED.owner_display_name,
			tags = EXCLUDED.tags,
			media_type = EXCLUDED.media_type,
			duration_seconds = EXCLUDED.duration_seconds
	`,
		video.ID,
		video.Title,
		video.Description,
		video.OwnerID,
		video.OwnerDisplayName,
		video.Tags,
		string(mediaType),
		video.DurationSeconds,
	)
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

func (q Query) rankExpression(_ string, _ string, _ string, _ string) string {
	switch q.Sort {
	case SortRecent:
		return "recency_score DESC, view_score DESC, engagement_score DESC"
	case SortViews:
		return "view_score DESC, recency_score DESC, engagement_score DESC"
	case SortEngagement:
		return "engagement_score DESC, view_score DESC, recency_score DESC"
	default:
		return "((0.65 * text_score) + (0.20 * recency_score) + (0.10 * view_score) + (0.05 * engagement_score)) DESC"
	}
}

func (d DurationFilter) bounds() (minSeconds, maxSeconds int) {
	switch d {
	case DurationShort:
		return 0, 4*60 - 1
	case DurationMedium:
		return 4 * 60, 20*60 - 1
	case DurationLong:
		return 20 * 60, 0
	default:
		return 0, 0
	}
}

func (u UploadDateFilter) cutoff() (time.Time, bool) {
	now := time.Now()
	switch u {
	case UploadDateToday:
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()), true
	case UploadDateWeek:
		return now.AddDate(0, 0, -7), true
	case UploadDateMonth:
		return now.AddDate(0, -1, 0), true
	case UploadDateYear:
		return now.AddDate(-1, 0, 0), true
	default:
		return time.Time{}, false
	}
}
