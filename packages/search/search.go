package search

import (
	"context"
	"time"
)

type Provider interface {
	Search(ctx context.Context, query Query) (Result, error)
	IndexVideo(ctx context.Context, video Video) error
	DeleteVideo(ctx context.Context, videoID string) error
	Health(ctx context.Context) error
}

type Query struct {
	Text   string
	Limit  int
	Offset int
}

type Result struct {
	Items []Video
	Total int
}

type Video struct {
	ID               string
	Title            string
	Description      string
	OwnerDisplayName string
	Tags             []string
	Score            float64
	IndexedAt        time.Time
}