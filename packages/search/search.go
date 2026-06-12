package search

import (
	"context"
	"time"
)

type MediaType string

const (
	MediaTypeVOD  MediaType = "vod"
	MediaTypeLive MediaType = "live"
)

type DurationFilter string

const (
	DurationAny    DurationFilter = ""
	DurationShort  DurationFilter = "short"
	DurationMedium DurationFilter = "medium"
	DurationLong   DurationFilter = "long"
)

type UploadDateFilter string

const (
	UploadDateAny   UploadDateFilter = ""
	UploadDateToday UploadDateFilter = "today"
	UploadDateWeek  UploadDateFilter = "week"
	UploadDateMonth UploadDateFilter = "month"
	UploadDateYear  UploadDateFilter = "year"
)

type Sort string

const (
	SortRelevance   Sort = "relevance"
	SortRecent      Sort = "recent"
	SortViews       Sort = "views"
	SortEngagement  Sort = "engagement"
)

type Provider interface {
	Search(ctx context.Context, query Query) (Result, error)
	Autocomplete(ctx context.Context, prefix string, limit int) ([]string, error)
	IndexVideo(ctx context.Context, video Video) error
	DeleteVideo(ctx context.Context, videoID string) error
	Health(ctx context.Context) error
}

type Query struct {
	Text       string          `json:"q"`
	Limit      int             `json:"limit,omitempty"`
	Offset     int             `json:"offset,omitempty"`
	Duration   DurationFilter  `json:"duration,omitempty"`
	UploadDate UploadDateFilter `json:"upload_date,omitempty"`
	MediaType  MediaType       `json:"media_type,omitempty"`
	OwnerID    string          `json:"owner_id,omitempty"`
	Sort       Sort            `json:"sort,omitempty"`
}

func (q Query) Normalized() Query {
	limit := q.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	offset := q.Offset
	if offset < 0 {
		offset = 0
	}

	sort := q.Sort
	if sort == "" {
		sort = SortRelevance
	}

	return Query{
		Text:       q.Text,
		Limit:      limit,
		Offset:     offset,
		Duration:   q.Duration,
		UploadDate: q.UploadDate,
		MediaType:  q.MediaType,
		OwnerID:    q.OwnerID,
		Sort:       sort,
	}
}

type Result struct {
	Items []ResultItem `json:"items"`
	Total int          `json:"total"`
	Query Query        `json:"query"`
}

type ResultItem struct {
	ID               string     `json:"id"`
	Title            string     `json:"title"`
	Description      string     `json:"description"`
	OwnerID          string     `json:"owner_id"`
	OwnerDisplayName string     `json:"owner_display_name"`
	Tags             []string   `json:"tags"`
	MediaType        MediaType  `json:"media_type"`
	DurationSeconds  *int       `json:"duration_seconds"`
	ViewCount        int64      `json:"view_count"`
	LikeCount        int64      `json:"like_count"`
	CreatedAt        time.Time  `json:"created_at"`
	PublishedAt      *time.Time `json:"published_at"`
	IndexedAt        time.Time  `json:"indexed_at"`
	TextScore        float64    `json:"text_score"`
	RecencyScore     float64    `json:"recency_score"`
	ViewScore        float64    `json:"view_score"`
	EngagementScore  float64    `json:"engagement_score"`
	Score            float64    `json:"score"`
}

type Video struct {
	ID               string
	Title            string
	Description      string
	OwnerID          string
	OwnerDisplayName string
	Tags             []string
	MediaType        MediaType
	DurationSeconds  *int
	ViewCount        int64
	LikeCount        int64
	CreatedAt        time.Time
	PublishedAt      *time.Time
	IndexedAt        time.Time
}
