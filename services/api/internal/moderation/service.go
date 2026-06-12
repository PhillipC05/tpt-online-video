package moderation

import (
	"context"
	"fmt"
)

// ─── Service ──────────────────────────────────────────────────────────────────

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Repository() *Repository {
	return s.repo
}

// ─── Report CRUD ──────────────────────────────────────────────────────────────

func (s *Service) CreateReport(ctx context.Context, reporterID, targetType, targetID, reason string) (*Report, error) {
	priority := 0
	return s.repo.CreateReport(ctx, reporterID, targetType, targetID, reason, priority)
}

func (s *Service) GetReport(ctx context.Context, reportID string) (*Report, error) {
	return s.repo.GetReport(ctx, reportID)
}

func (s *Service) ListReports(ctx context.Context, filter ReportFilter) ([]*Report, int, error) {
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	return s.repo.ListReports(ctx, filter)
}

func (s *Service) AssignReport(ctx context.Context, reportID, assigneeID string) error {
	return s.repo.AssignReport(ctx, reportID, assigneeID)
}

func (s *Service) UnassignReport(ctx context.Context, reportID string) error {
	return s.repo.UnassignReport(ctx, reportID)
}

func (s *Service) ResolveReport(ctx context.Context, reportID, resolvedBy string) error {
	return s.repo.ResolveReport(ctx, reportID, resolvedBy, ReportStatusResolved)
}

func (s *Service) DismissReport(ctx context.Context, reportID, resolvedBy string) error {
	return s.repo.ResolveReport(ctx, reportID, resolvedBy, ReportStatusDismissed)
}

// ─── Admin Notes ──────────────────────────────────────────────────────────────

func (s *Service) SetAdminNotes(ctx context.Context, reportID, notes string) error {
	return s.repo.SetAdminNotes(ctx, reportID, notes)
}

// ─── Appeals ──────────────────────────────────────────────────────────────────

func (s *Service) SubmitAppeal(ctx context.Context, reportID, reason string) error {
	return s.repo.SubmitAppeal(ctx, reportID, reason)
}

func (s *Service) ResolveAppeal(ctx context.Context, reportID, resolvedBy, decision, notes string) error {
	return s.repo.ResolveAppeal(ctx, reportID, resolvedBy, decision, notes)
}

// ─── Moderation Actions ───────────────────────────────────────────────────────

type ExecuteActionResult struct {
	Action *ModerationAction
}

func (s *Service) ExecuteAction(ctx context.Context, actorID string, reportID *string, targetType string, targetID string, actionType ModerationActionType, reason *string, metadata map[string]interface{}, adminNotes *string, ipAddress, userAgent *string) (*ExecuteActionResult, error) {
	action, err := s.repo.CreateAction(ctx, actorID, reportID, targetType, targetID, actionType, reason, metadata, adminNotes)
	if err != nil {
		return nil, fmt.Errorf("create action: %w", err)
	}

	// Apply the action to the target
	if err := s.applyActionToTarget(ctx, actionType, targetType, targetID); err != nil {
		return nil, fmt.Errorf("apply action: %w", err)
	}

	// Create audit log entry
	auditMeta := map[string]interface{}{
		"action_type": string(actionType),
		"target_type": targetType,
		"target_id":   targetID,
	}
	if reason != nil {
		auditMeta["reason"] = *reason
	}

	s.repo.CreateAuditLog(ctx, &actorID,
		fmt.Sprintf("moderation:%s", string(actionType)),
		&targetType, &targetID,
		auditMeta, ipAddress, userAgent)

	return &ExecuteActionResult{Action: action}, nil
}

func (s *Service) ReverseAction(ctx context.Context, actionID string, reversedBy, reason, ipAddress, userAgent *string) error {
	action, err := s.repo.GetActionByID(ctx, actionID)
	if err != nil {
		return err
	}
	if action == nil {
		return fmt.Errorf("action not found")
	}
	if action.ReversedAt != nil {
		return fmt.Errorf("action already reversed")
	}

	if err := s.repo.ReverseAction(ctx, actionID, *reversedBy, *reason); err != nil {
		return err
	}

	// Restore the content if it was a hide/delete action
	if action.ActionType == ActionHideContent || action.ActionType == ActionDeleteVideo || action.ActionType == ActionRemoveComment {
		_ = s.applyActionToTarget(ctx, ActionRestoreContent, action.TargetType, action.TargetID)
	}

	// Audit log the reversal
	s.repo.CreateAuditLog(ctx, reversedBy,
		fmt.Sprintf("moderation:reverse:%s", string(action.ActionType)),
		&action.TargetType, &action.TargetID,
		map[string]interface{}{
			"original_action_id": actionID,
			"reason":             *reason,
		}, ipAddress, userAgent)

	return nil
}

func (s *Service) GetAction(ctx context.Context, actionID string) (*ModerationAction, error) {
	return s.repo.GetActionByID(ctx, actionID)
}

func (s *Service) ListActions(ctx context.Context, filter ActionFilter) ([]*ModerationAction, int, error) {
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	return s.repo.ListActions(ctx, filter)
}

// ─── Audit Log ────────────────────────────────────────────────────────────────

func (s *Service) ListAuditLog(ctx context.Context, filter AuditFilter) ([]*AuditLogEntry, int, error) {
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	return s.repo.ListAuditLog(ctx, filter)
}

func (s *Service) CreateAuditLog(ctx context.Context, actorID *string, action string, targetType, targetID *string, metadata map[string]interface{}, ipAddress, userAgent *string) error {
	return s.repo.CreateAuditLog(ctx, actorID, action, targetType, targetID, metadata, ipAddress, userAgent)
}

// ─── Internal helpers ─────────────────────────────────────────────────────────

func (s *Service) applyActionToTarget(ctx context.Context, actionType ModerationActionType, targetType string, targetID string) error {
	switch actionType {
	case ActionHideContent:
		// Mark as hidden where applicable
		switch targetType {
		case "video":
			_, err := s.repo.db.Exec(ctx,
				`UPDATE videos SET visibility = 'private' WHERE id = $1 AND deleted_at IS NULL`, targetID)
			return err
		case "comment":
			_, err := s.repo.db.Exec(ctx,
				`UPDATE comments SET status = 'hidden' WHERE id = $1 AND deleted_at IS NULL`, targetID)
			return err
		}

	case ActionUnpublishVideo:
		if targetType == "video" {
			_, err := s.repo.db.Exec(ctx,
				`UPDATE videos SET visibility = 'unlisted', published_at = NULL WHERE id = $1 AND deleted_at IS NULL`, targetID)
			return err
		}

	case ActionDeleteVideo:
		if targetType == "video" {
			_, err := s.repo.db.Exec(ctx,
				`UPDATE videos SET deleted_at = NOW(), visibility = 'removed' WHERE id = $1 AND deleted_at IS NULL`, targetID)
			return err
		}

	case ActionRemoveComment:
		if targetType == "comment" {
			_, err := s.repo.db.Exec(ctx,
				`UPDATE comments SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, targetID)
			return err
		}

	case ActionSuspendUser:
		if targetType == "user" {
			_, err := s.repo.db.Exec(ctx,
				`UPDATE users SET status = 'suspended' WHERE id = $1 AND deleted_at IS NULL`, targetID)
			return err
		}

	case ActionBanUser:
		if targetType == "user" {
			_, err := s.repo.db.Exec(ctx,
				`UPDATE users SET status = 'banned' WHERE id = $1 AND deleted_at IS NULL`, targetID)
			return err
		}

	case ActionRestoreContent:
		switch targetType {
		case "video":
			_, err := s.repo.db.Exec(ctx,
				`UPDATE videos SET deleted_at = NULL, visibility = 'public' WHERE id = $1 AND deleted_at IS NOT NULL`, targetID)
			return err
		case "comment":
			_, err := s.repo.db.Exec(ctx,
				`UPDATE comments SET status = 'visible', deleted_at = NULL WHERE id = $1 AND deleted_at IS NOT NULL`, targetID)
			return err
		case "user":
			_, err := s.repo.db.Exec(ctx,
				`UPDATE users SET status = 'active' WHERE id = $1 AND status IN ('suspended', 'banned')`, targetID)
			return err
		}

	case ActionLockChat:
		// Live chat lock would be handled by live service
		return nil

	case ActionTerminateStream:
		// Live stream termination would be handled by live service
		return nil
	}

	return nil
}