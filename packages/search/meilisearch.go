package search

import (
	"context"
	"fmt"
	"time"

	"github.com/meilisearch/meilisearch-go"
)

// MeilisearchProvider implements the search.Provider interface using Meilisearch.
type MeilisearchProvider struct {
	client  meilisearch.ServiceManager
	indexID string // the Meilisearch index name for videos
}

// NewMeilisearchProvider creates a new MeilisearchProvider.
// host is the Meilisearch server URL (e.g. "http://localhost:7700").
// apiKey is the optional master/private API key (empty string for no auth).
// indexID is the Meilisearch index name (e.g. "videos").
func NewMeilisearchProvider(host, apiKey, indexID string) *MeilisearchProvider {
	client := meilisearch.NewClient(meilisearch.ClientConfig{
		Host:   host,
		APIKey: apiKey,
	})
	return &MeilisearchProvider{
		client:  client,
		indexID: indexID,
	}
}

type meiliDocument struct {
	ID               string   `json:"id"`
	Title            string   `json:"title"`
	Description      string   `json:"description"`
	OwnerID          string   `json:"owner_id"`
	OwnerDisplayName string   `json:"owner_display_name"`
	Tags             []string `json:"tags"`
	MediaType        string   `json:"media_type"`
	DurationSeconds  *int     `json:"duration_seconds"`
	ViewCount        int64    `json:"view_count"`
	LikeCount        int64    `json:"like_count"`
	CreatedAt        int64    `json:"created_at"`        // unix timestamp
	PublishedAt      *int64   `json:"published_at,omitempty"` // unix timestamp or nil
	IndexedAt        int64    `json:"indexed_at"`        // unix timestamp
}

func toMeiliDoc(v Video) meiliDocument {
	doc := meiliDocument{
		ID:               v.ID,
		Title:            v.Title,
		Description:      v.Description,
		OwnerID:          v.OwnerID,
		OwnerDisplayName: v.OwnerDisplayName,
		Tags:             v.Tags,
		MediaType:        string(v.MediaType),
		DurationSeconds:  v.DurationSeconds,
		ViewCount:        v.ViewCount,
		LikeCount:        v.LikeCount,
		CreatedAt:        v.CreatedAt.Unix(),
		IndexedAt:        v.IndexedAt.Unix(),
	}
	if v.PublishedAt != nil {
		ts := v.PublishedAt.Unix()
		doc.PublishedAt = &ts
	}
	if doc.MediaType == "" {
		doc.MediaType = string(MediaTypeVOD)
	}
	return doc
}

func (p *MeilisearchProvider) Search(ctx context.Context, query Query) (Result, error) {
	q := query.Normalized()

	var filterParts []string

	// Duration filter
	if q.Duration != DurationAny {
		minS, maxS := q.Duration.bounds()
		if minS > 0 {
			filterParts = append(filterParts, fmt.Sprintf("duration_seconds >= %d", minS))
		}
		if maxS > 0 {
			filterParts = append(filterParts, fmt.Sprintf("duration_seconds <= %d", maxS))
		}
	}

	// Upload date filter
	if q.UploadDate != UploadDateAny {
		cutoff, ok := q.UploadDate.cutoff()
		if ok {
			filterParts = append(filterParts, fmt.Sprintf("published_at >= %d", cutoff.Unix()))
		}
	}

	// Media type filter
	if q.MediaType != "" {
		filterParts = append(filterParts, fmt.Sprintf("media_type = '%s'", string(q.MediaType)))
	}

	// Owner filter
	if q.OwnerID != "" {
		filterParts = append(filterParts, fmt.Sprintf("owner_id = '%s'", q.OwnerID))
	}

	// Sort
	sortOrder := []string{}
	switch q.Sort {
	case SortRecent:
		sortOrder = []string{"published_at:desc", "created_at:desc"}
	case SortViews:
		sortOrder = []string{"view_count:desc"}
	default:
		sortOrder = []string{} // Meilisearch defaults to relevance
	}

	searchRequest := &meilisearch.SearchRequest{
		Limit:  int64(q.Limit),
		Offset: int64(q.Offset),
		Sort:   sortOrder,
	}

	if len(filterParts) > 0 {
		filterStr := ""
		for i, part := range filterParts {
			if i > 0 {
				filterStr += " AND "
			}
			filterStr += part
		}
		searchRequest.Filter = filterStr
	}

	resp, err := p.client.Index(p.indexID).Search(q.Text, searchRequest)
	if err != nil {
		return Result{}, fmt.Errorf("meilisearch search: %w", err)
	}

	hits, ok := resp.Hits.([]any)
	if !ok {
		// If no hits or type mismatch, return empty
		return Result{Items: []ResultItem{}, Total: 0, Query: q}, nil
	}

	items := make([]ResultItem, 0, len(hits))
	for _, hit := range hits {
		hitMap, ok := hit.(map[string]any)
		if !ok {
			continue
		}

		item := ResultItem{
			ID:               getStringField(hitMap, "id"),
			Title:            getStringField(hitMap, "title"),
			Description:      getStringField(hitMap, "description"),
			OwnerID:          getStringField(hitMap, "owner_id"),
			OwnerDisplayName: getStringField(hitMap, "owner_display_name"),
			Tags:             getStringSliceField(hitMap, "tags"),
			MediaType:        MediaType(getStringField(hitMap, "media_type")),
			DurationSeconds:  getIntPtrField(hitMap, "duration_seconds"),
			ViewCount:        getInt64Field(hitMap, "view_count"),
			LikeCount:        getInt64Field(hitMap, "like_count"),
			CreatedAt:        time.Unix(getInt64Field(hitMap, "created_at"), 0),
			IndexedAt:        time.Unix(getInt64Field(hitMap, "indexed_at"), 0),
		}

		if pubTS, ok := hitMap["published_at"].(float64); ok && int64(pubTS) > 0 {
			t := time.Unix(int64(pubTS), 0)
			item.PublishedAt = &t
		}

		// Meilisearch provides its own score under _rankingScore
		if score, ok := hitMap["_rankingScore"].(float64); ok {
			item.Score = score
		}

		items = append(items, item)
	}

	return Result{
		Items: items,
		Total: int(resp.EstimatedTotalHits),
		Query: q,
	}, nil
}

func (p *MeilisearchProvider) Autocomplete(ctx context.Context, prefix string, limit int) ([]string, error) {
	if prefix == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 20 {
		limit = 10
	}

	// Meilisearch doesn't have a native prefix autocomplete endpoint, but we
	// can use a search with the prefix parameter  to get title matches.
	searchRequest := &meilisearch.SearchRequest{
		Limit:  int64(limit),
		Sort:   []string{},
	}

	resp, err := p.client.Index(p.indexID).Search(prefix, searchRequest)
	if err != nil {
		return nil, fmt.Errorf("meilisearch autocomplete: %w", err)
	}

	hits, ok := resp.Hits.([]any)
	if !ok {
		return nil, nil
	}

	suggestions := make([]string, 0, len(hits))
	seen := map[string]bool{}
	for _, hit := range hits {
		hitMap, ok := hit.(map[string]any)
		if !ok {
			continue
		}
		title := getStringField(hitMap, "title")
		if title != "" && !seen[title] {
			seen[title] = true
			suggestions = append(suggestions, title)
		}
	}
	return suggestions, nil
}

func (p *MeilisearchProvider) IndexVideo(ctx context.Context, video Video) error {
	if video.ID == "" || video.Title == "" {
		return fmt.Errorf("video id and title are required")
	}
	doc := toMeiliDoc(video)
	_, err := p.client.Index(p.indexID).AddDocuments([]meiliDocument{doc})
	if err != nil {
		return fmt.Errorf("meilisearch index video: %w", err)
	}
	return nil
}

func (p *MeilisearchProvider) DeleteVideo(ctx context.Context, videoID string) error {
	_, err := p.client.Index(p.indexID).DeleteDocument(videoID)
	if err != nil {
		return fmt.Errorf("meilisearch delete video: %w", err)
	}
	return nil
}

func (p *MeilisearchProvider) Health(ctx context.Context) error {
	_, err := p.client.Health()
	if err != nil {
		return fmt.Errorf("meilisearch health: %w", err)
	}
	return nil
}

// ── helper field extractors ──────────────────────────────────────────────────

func getStringField(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

func getStringSliceField(m map[string]any, key string) []string {
	v, ok := m[key]
	if !ok {
		return nil
	}
	raw, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func getInt64Field(m map[string]any, key string) int64 {
	v, ok := m[key]
	if !ok {
		return 0
	}
	f, ok := v.(float64)
	if !ok {
		return 0
	}
	return int64(f)
}

func getIntPtrField(m map[string]any, key string) *int {
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	f, ok := v.(float64)
	if !ok {
		return nil
	}
	val := int(f)
	return &val
}