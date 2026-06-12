package moderation

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Test helpers

func newTestRepo(t *testing.T) *Repository {
	t.Helper()

	// Try to connect to a test database; skip if unavailable
	pool, err := pgxpool.New(context.Background(),
		"postgres://postgres:postgres@localhost:5432/tpt_test?sslmode=disable")
	if err != nil {
		t.Skip("test database not available:", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		t.Skip("test database not reachable:", err)
	}

	return NewRepository(pool)
}

func TestCreateReport(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	report, err := repo.CreateReport(ctx, "00000000-0000-0000-0000-000000000001", "video",
		"00000000-0000-0000-0000-000000000002", "Inappropriate content", 0)
	if err != nil {
		t.Fatalf("CreateReport failed: %v", err)
	}

	if report.ID == "" {
		t.Error("expected report ID to be generated")
	}
	if report.Status != ReportStatusOpen {
		t.Errorf("expected status 'open', got %q", report.Status)
	}
	if report.TargetType != "video" {
		t.Errorf("expected target_type 'video', got %q", report.TargetType)
	}
}

func TestReportLifecycle(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	// Create
	report, err := repo.CreateReport(ctx, "00000000-0000-0000-0000-000000000001",
		"comment", "00000000-0000-0000-0000-000000000003",
		"Spam comment", 5)
	if err != nil {
		t.Fatalf("CreateReport failed: %v", err)
	}

	// Assign
	err = repo.AssignReport(ctx, report.ID, "00000000-0000-0000-0000-000000000004")
	if err != nil {
		t.Fatalf("AssignReport failed: %v", err)
	}

	// Check assigned status
	fetched, err := repo.GetReport(ctx, report.ID)
	if err != nil {
		t.Fatalf("GetReport failed: %v", err)
	}
	if fetched.Status != ReportStatusAssigned {
		t.Errorf("expected status 'assigned', got %q", fetched.Status)
	}
	if fetched.AssigneeID == nil || *fetched.AssigneeID != "00000000-0000-0000-0000-000000000004" {
		t.Error("expected assignee to be set")
	}

	// Resolve
	err = repo.ResolveReport(ctx, report.ID, "00000000-0000-0000-0000-000000000004", ReportStatusResolved)
	if err != nil {
		t.Fatalf("ResolveReport failed: %v", err)
	}

	fetched, err = repo.GetReport(ctx, report.ID)
	if err != nil {
		t.Fatalf("GetReport failed: %v", err)
	}
	if fetched.Status != ReportStatusResolved {
		t.Errorf("expected status 'resolved', got %q", fetched.Status)
	}
	if fetched.ResolvedAt == nil {
		t.Error("expected resolved_at to be set")
	}
}

func TestAppealFlow(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	// Create and resolve a report
	report, err := repo.CreateReport(ctx, "00000000-0000-0000-0000-000000000001",
		"video", "00000000-0000-0000-0000-000000000005",
		"Copyright violation", 3)
	if err != nil {
		t.Fatalf("CreateReport failed: %v", err)
	}

	err = repo.ResolveReport(ctx, report.ID, "00000000-0000-0000-0000-000000000004", ReportStatusResolved)
	if err != nil {
		t.Fatalf("ResolveReport failed: %v", err)
	}

	// Submit appeal
	err = repo.SubmitAppeal(ctx, report.ID, "This was a mistake, content is original")
	if err != nil {
		t.Fatalf("SubmitAppeal failed: %v", err)
	}

	fetched, err := repo.GetReport(ctx, report.ID)
	if err != nil {
		t.Fatalf("GetReport failed: %v", err)
	}
	if fetched.AppealStatus == nil || *fetched.AppealStatus != AppealStatusPending {
		t.Errorf("expected appeal_status 'pending', got %v", fetched.AppealStatus)
	}
	if fetched.AppealReason == nil || *fetched.AppealReason != "This was a mistake, content is original" {
		t.Errorf("appeal_reason not set correctly")
	}

	// Resolve appeal - granted
	err = repo.ResolveAppeal(ctx, report.ID, "00000000-0000-0000-0000-000000000004", "granted", "Appeal accepted")
	if err != nil {
		t.Fatalf("ResolveAppeal failed: %v", err)
	}

	fetched, err = repo.GetReport(ctx, report.ID)
	if err != nil {
		t.Fatalf("GetReport failed: %v", err)
	}
	if fetched.AppealDecision == nil || *fetched.AppealDecision != "granted" {
		t.Errorf("expected appeal_decision 'granted', got %v", fetched.AppealDecision)
	}
}

func TestModerationAction(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	action, err := repo.CreateAction(ctx, "00000000-0000-0000-0000-000000000004", nil,
		"video", "00000000-0000-0000-0000-000000000006",
		ActionHideContent, strPtr("Inappropriate content"),
		map[string]interface{}{"reason": "violates guidelines"}, nil)
	if err != nil {
		t.Fatalf("CreateAction failed: %v", err)
	}

	if action.ID == "" {
		t.Error("expected action ID to be generated")
	}
	if action.ActionType != ActionHideContent {
		t.Errorf("expected action_type 'hide_content', got %q", action.ActionType)
	}
	if action.Reason == nil || *action.Reason != "Inappropriate content" {
		t.Error("reason not set correctly")
	}

	// List actions
	actions, total, err := repo.ListActions(ctx, ActionFilter{
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("ListActions failed: %v", err)
	}
	if total < 1 {
		t.Errorf("expected at least 1 action, got %d", total)
	}
	if len(actions) < 1 {
		t.Error("expected at least 1 action in list")
	}
}

func TestAuditLog(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	actorID := "00000000-0000-0000-0000-000000000004"
	targetType := "video"
	targetID := "00000000-0000-0000-0000-000000000007"
	ipAddr := "192.168.1.1"
	ua := "test-agent"

	err := repo.CreateAuditLog(ctx, &actorID, "moderation:hide_content",
		&targetType, &targetID,
		map[string]interface{}{"reason": "test"},
		&ipAddr, &ua)
	if err != nil {
		t.Fatalf("CreateAuditLog failed: %v", err)
	}

	entries, total, err := repo.ListAuditLog(ctx, AuditFilter{
		ActorID: &actorID,
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("ListAuditLog failed: %v", err)
	}
	if total < 1 {
		t.Errorf("expected at least 1 audit entry, got %d", total)
	}
	if len(entries) < 1 {
		t.Error("expected at least 1 entry")
	}
	if entries[0].Action != "moderation:hide_content" {
		t.Errorf("expected action 'moderation:hide_content', got %q", entries[0].Action)
	}
}

func TestListReportsFiltered(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	// Create a few reports
	repo.CreateReport(ctx, "00000000-0000-0000-0000-000000000001", "video",
		"00000000-0000-0000-0000-000000000008", "Test report 1", 0)
	repo.CreateReport(ctx, "00000000-0000-0000-0000-000000000001", "comment",
		"00000000-0000-0000-0000-000000000009", "Test report 2", 0)
	repo.CreateReport(ctx, "00000000-0000-0000-0000-000000000001", "user",
		"00000000-0000-0000-0000-000000000010", "Test report 3", 0)

	// Filter by target_type
	tt := "video"
	reports, total, err := repo.ListReports(ctx, ReportFilter{
		TargetType: &tt,
		Limit:      10,
	})
	if err != nil {
		t.Fatalf("ListReports failed: %v", err)
	}
	if total < 1 {
		t.Errorf("expected at least 1 video report, got %d", total)
	}
	for _, r := range reports {
		if r.TargetType != "video" {
			t.Errorf("expected all reports to be 'video', got %q", r.TargetType)
		}
	}
}

func TestReverseAction(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	action, err := repo.CreateAction(ctx, "00000000-0000-0000-0000-000000000004", nil,
		"video", "00000000-0000-0000-0000-000000000011",
		ActionDeleteVideo, strPtr("Test delete"),
		map[string]interface{}{}, nil)
	if err != nil {
		t.Fatalf("CreateAction failed: %v", err)
	}

	err = repo.ReverseAction(ctx, action.ID, "00000000-0000-0000-0000-000000000004", "Reversed in error")
	if err != nil {
		t.Fatalf("ReverseAction failed: %v", err)
	}

	// Verify it's marked reversed
	reversed, err := repo.GetActionByID(ctx, action.ID)
	if err != nil {
		t.Fatalf("GetActionByID failed: %v", err)
	}
	if reversed.ReversedAt == nil {
		t.Error("expected reversed_at to be set")
	}
	if reversed.ReversedBy == nil || *reversed.ReversedBy != "00000000-0000-0000-0000-000000000004" {
		t.Error("expected reversed_by to be set")
	}
}

// helpers

func strPtr(s string) *string { return &s }