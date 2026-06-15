package search

import (
	"testing"
	"time"
)

// --- Query.Normalized ---

func TestQuery_Normalized_Defaults(t *testing.T) {
	q := Query{}
	n := q.Normalized()

	if n.Limit != 20 {
		t.Errorf("expected default Limit=20, got %d", n.Limit)
	}
	if n.Offset != 0 {
		t.Errorf("expected default Offset=0, got %d", n.Offset)
	}
	if n.Sort != SortRelevance {
		t.Errorf("expected default Sort=%q, got %q", SortRelevance, n.Sort)
	}
}

func TestQuery_Normalized_ClampsLimit(t *testing.T) {
	cases := []struct {
		input    int
		expected int
	}{
		{-1, 20},
		{0, 20},
		{50, 50},
		{100, 100},
		{101, 100},
		{999, 100},
	}
	for _, c := range cases {
		q := Query{Limit: c.input}
		n := q.Normalized()
		if n.Limit != c.expected {
			t.Errorf("Normalized().Limit for input %d: got %d, want %d", c.input, n.Limit, c.expected)
		}
	}
}

func TestQuery_Normalized_ClampsNegativeOffset(t *testing.T) {
	q := Query{Offset: -5}
	n := q.Normalized()
	if n.Offset != 0 {
		t.Errorf("expected Offset=0 for negative input, got %d", n.Offset)
	}
}

func TestQuery_Normalized_PositiveOffset(t *testing.T) {
	q := Query{Offset: 40}
	n := q.Normalized()
	if n.Offset != 40 {
		t.Errorf("expected Offset=40, got %d", n.Offset)
	}
}

func TestQuery_Normalized_PreservesFields(t *testing.T) {
	q := Query{
		Text:       "golang",
		Limit:      10,
		Offset:     5,
		Duration:   DurationShort,
		UploadDate: UploadDateWeek,
		MediaType:  MediaTypeVOD,
		OwnerID:    "user-123",
		Sort:       SortViews,
	}
	n := q.Normalized()

	if n.Text != "golang" {
		t.Errorf("Text: got %q, want %q", n.Text, "golang")
	}
	if n.Duration != DurationShort {
		t.Errorf("Duration: got %q, want %q", n.Duration, DurationShort)
	}
	if n.UploadDate != UploadDateWeek {
		t.Errorf("UploadDate: got %q, want %q", n.UploadDate, UploadDateWeek)
	}
	if n.MediaType != MediaTypeVOD {
		t.Errorf("MediaType: got %q, want %q", n.MediaType, MediaTypeVOD)
	}
	if n.OwnerID != "user-123" {
		t.Errorf("OwnerID: got %q, want %q", n.OwnerID, "user-123")
	}
	if n.Sort != SortViews {
		t.Errorf("Sort: got %q, want %q", n.Sort, SortViews)
	}
}

func TestQuery_Normalized_DefaultsSortWhenEmpty(t *testing.T) {
	q := Query{Sort: ""}
	n := q.Normalized()
	if n.Sort != SortRelevance {
		t.Errorf("expected Sort=%q when empty, got %q", SortRelevance, n.Sort)
	}
}

// --- DurationFilter.bounds ---

func TestDurationFilter_Bounds(t *testing.T) {
	cases := []struct {
		filter   DurationFilter
		minSec   int
		maxSec   int
	}{
		{DurationAny, 0, 0},
		{DurationShort, 0, 4*60 - 1},
		{DurationMedium, 4 * 60, 20*60 - 1},
		{DurationLong, 20 * 60, 0},
	}
	for _, c := range cases {
		min, max := c.filter.bounds()
		if min != c.minSec {
			t.Errorf("bounds(%q): min=%d, want %d", c.filter, min, c.minSec)
		}
		if max != c.maxSec {
			t.Errorf("bounds(%q): max=%d, want %d", c.filter, max, c.maxSec)
		}
	}
}

// --- UploadDateFilter.cutoff ---

func TestUploadDateFilter_Cutoff(t *testing.T) {
	before := time.Now().Add(-time.Second)

	cases := []struct {
		filter    UploadDateFilter
		expectOK  bool
		maxAge    time.Duration
	}{
		{UploadDateAny, false, 0},
		{UploadDateToday, true, 24 * time.Hour},
		{UploadDateWeek, true, 7 * 24 * time.Hour},
		{UploadDateMonth, true, 32 * 24 * time.Hour},
		{UploadDateYear, true, 366 * 24 * time.Hour},
	}

	for _, c := range cases {
		cutoff, ok := c.filter.cutoff()
		if ok != c.expectOK {
			t.Errorf("cutoff(%q): ok=%v, want %v", c.filter, ok, c.expectOK)
			continue
		}
		if !c.expectOK {
			continue
		}

		// cutoff should be in the past
		if !cutoff.Before(before.Add(time.Second)) {
			t.Errorf("cutoff(%q): expected a past time, got %v", c.filter, cutoff)
		}
		// cutoff should not be more than the expected max age ago
		if time.Since(cutoff) > c.maxAge+time.Hour {
			t.Errorf("cutoff(%q): cutoff %v is too far in the past (max %v ago)", c.filter, cutoff, c.maxAge)
		}
	}
}

// --- Query.rankExpression ---

func TestQuery_RankExpression(t *testing.T) {
	cases := []struct {
		sort     Sort
		contains string
	}{
		{SortRecent, "recency_score"},
		{SortViews, "view_score"},
		{SortEngagement, "engagement_score"},
		{SortRelevance, "text_score"},
		{"", "text_score"}, // unknown defaults to relevance
	}

	for _, c := range cases {
		q := Query{Sort: c.sort}
		expr := q.rankExpression("text_score", "recency_score", "view_score", "engagement_score")
		if len(expr) == 0 {
			t.Errorf("rankExpression(%q): expected non-empty expression", c.sort)
		}
		_ = expr // we test it is non-empty and a valid non-panicking call
	}
}

// --- MediaType / Sort constants ---

func TestMediaTypeConstants(t *testing.T) {
	if MediaTypeVOD == "" {
		t.Error("MediaTypeVOD should not be empty")
	}
	if MediaTypeLive == "" {
		t.Error("MediaTypeLive should not be empty")
	}
	if MediaTypeVOD == MediaTypeLive {
		t.Error("MediaTypeVOD and MediaTypeLive should differ")
	}
}

func TestSortConstants(t *testing.T) {
	sorts := []Sort{SortRelevance, SortRecent, SortViews, SortEngagement}
	seen := make(map[Sort]bool)
	for _, s := range sorts {
		if s == "" {
			t.Errorf("Sort constant should not be empty")
		}
		if seen[s] {
			t.Errorf("duplicate Sort constant: %q", s)
		}
		seen[s] = true
	}
}
