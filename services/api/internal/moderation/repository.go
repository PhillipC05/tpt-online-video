package moderation

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ─── Types ────────────────────────────────────────────────────────────────────

type ReportStatus string

const (
	ReportStatusOpen      ReportStatus = "open"
	ReportStatusAssigned  ReportStatus = "assigned"
	ReportStatusResolved  ReportStatus = "resolved"
	ReportStatusDismissed ReportStatus = "dismissed"
)

type AppealStatus string

const (
	AppealStatusPending AppealStatus = "pending"
	AppealStatusGranted AppealStatus = "granted"
	AppealStatusDenied  AppealStatus = "denied"
)

type Report struct {
	ID             string       `json:"id"`
	ReporterID     string       `json:"reporter_id"`
	AssigneeID     *string      `json:"assignee_id,omitempty"`
	TargetType     string       `json:"target_type"`
	TargetID       string       `json:"target_id"`
	Reason         string       `json:"reason"`
	Status         ReportStatus `json:"status"`
	Priority       int          `json:"priority"`
	AdminNotes     *string      `json:"admin_notes,omitempty"`
	ResolvedBy     *string      `json:"resolved_by,omitempty"`
	AppealStatus   *AppealStatus `json:"appeal_status,omitempty"`
	AppealReason   *string      `json:"appeal_reason,omitempty"`
	AppealedAt     *time.Time   `json:"appealed_at,omitempty"`
	AppealDecision *string      `json:"appeal_decision,omitempty"`
	AppealResolvedBy *string    `json:"appeal_resolved_by,omitempty"`
	AppealResolvedAt *time.Time `json:"appeal_resolved_at,omitempty"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
	ResolvedAt     *time.Time   `json:"resolved_at,omitempty"`

	// Joined fields
	ReporterName string `json:"reporter_name,omitempty"`
	AssigneeName string `json:"assignee_name,omitempty"`
}

type ModerationActionType string

const (
	ActionHideContent        ModerationActionType = "hide_content"
	ActionUnpublishVideo     ModerationActionType = "unpublish_video"
	ActionDeleteVideo        ModerationActionType = "delete_video"
	ActionRemoveComment      ModerationActionType = "remove_comment"
	ActionTerminateStream    ModerationActionType = "terminate_live_stream"
	ActionLockChat           ModerationActionType = "lock_live_chat"
	ActionSuspendUser        ModerationActionType = "suspend_user"
	ActionBanUser            ModerationActionType = "ban_user"
	ActionRestoreContent     ModerationActionType = "restore_content"
)

type ModerationAction struct {
	ID            string               `json:"id"`
	ActorID       string               `json:"actor_id"`
	ReportID      *string              `json:"report_id,omitempty"`
	TargetType    string               `json:"target_type"`
	TargetID      string               `json:"target_id"`
	ActionType    ModerationActionType  `json:"action_type"`
	Reason        *string              `json:"reason,omitempty"`
	Metadata      json.RawMessage      `json:"metadata"`
	AdminNotes    *string              `json:"admin_notes,omitempty"`
	ReversedBy    *string              `json:"reversed_by,omitempty"`
	ReversedAt    *time.Time           `json:"reversed_at,omitempty"`
	ReversedReason *string             `json:"reversed_reason,omitempty"`
	CreatedAt     time.Time            `json:"created_at"`

	ActorName string `json:"actor_name,omitempty"`
}

type AuditLogEntry struct {
	ID         string          `json:"id"`
	ActorID    *string         `json:"actor_id,omitempty"`
	Action     string          `json:"action"`
	TargetType *string         `json:"target_type,omitempty"`
	TargetID   *string         `json:"target_id,omitempty"`
	Metadata   json.RawMessage `json:"metadata"`
	IPAddress  *string         `json:"ip_address,omitempty"`
	UserAgent  *string         `json:"user_agent,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
	ActorName  string          `json:"actor_name,omitempty"`
}

type ReportFilter struct {
	Status     *ReportStatus `json:"status,omitempty"`
	TargetType *string       `json:"target_type,omitempty"`
	AssigneeID *string       `json:"assignee_id,omitempty"`
	PriorityGt *int          `json:"priority_gt,omitempty"`
	Offset     int           `json:"offset"`
	Limit      int           `json:"limit"`
}

type ActionFilter struct {
	TargetType *string `json:"target_type,omitempty"`
	TargetID   *string `json:"target_id,omitempty"`
	ActorID    *string `json:"actor_id,omitempty"`
	Offset     int     `json:"offset"`
	Limit      int     `json:"limit"`
}

type AuditFilter struct {
	ActorID    *string    `json:"actor_id,omitempty"`
	Action     *string    `json:"action,omitempty"`
	TargetType *string    `json:"target_type,omitempty"`
	TargetID   *string    `json:"target_id,omitempty"`
	Since      *time.Time `json:"since,omitempty"` // inclusive lower bound on created_at
	Until      *time.Time `json:"until,omitempty"` // inclusive upper bound on created_at
	Offset     int        `json:"offset"`
	Limit      int        `json:"limit"`
}

// ─── Repository ───────────────────────────────────────────────────────────────

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// ─── Reports ──────────────────────────────────────────────────────────────────

func (r *Repository) CreateReport(ctx context.Context, reporterID, targetType, targetID, reason string, priority int) (*Report, error) {
	report := &Report{}
	err := r.db.QueryRow(ctx,
		`INSERT INTO moderation_reports (reporter_id, target_type, target_id, reason, priority)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, reporter_id, target_type, target_id, reason, status, priority, created_at, updated_at`,
		reporterID, targetType, targetID, reason, priority,
	).Scan(&report.ID, &report.ReporterID, &report.TargetType, &report.TargetID,
		&report.Reason, &report.Status, &report.Priority, &report.CreatedAt, &report.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return report, nil
}

func (r *Repository) GetReport(ctx context.Context, reportID string) (*Report, error) {
	report := &Report{}
	var assigneeID, resolvedBy, adminNotes *string
	var appealStatus *string
	var appealReason, appealDecision, appealResolvedBy *string
	var appealedAt, appealResolvedAt, resolvedAt *time.Time

	err := r.db.QueryRow(ctx,
		`SELECT mr.id, mr.reporter_id, mr.assignee_id, mr.target_type, mr.target_id,
		        mr.reason, mr.status, mr.priority, mr.admin_notes, mr.resolved_by,
		        mr.appeal_status, mr.appeal_reason, mr.appealed_at,
		        mr.appeal_decision, mr.appeal_resolved_by, mr.appeal_resolved_at,
		        mr.created_at, mr.updated_at, mr.resolved_at,
		        u.display_name AS reporter_name,
		        COALESCE(a.display_name, '') AS assignee_name
		 FROM moderation_reports mr
		 JOIN users u ON u.id = mr.reporter_id
		 LEFT JOIN users a ON a.id = mr.assignee_id
		 WHERE mr.id = $1`, reportID,
	).Scan(&report.ID, &report.ReporterID, &assigneeID, &report.TargetType, &report.TargetID,
		&report.Reason, &report.Status, &report.Priority, &adminNotes, &resolvedBy,
		&appealStatus, &appealReason, &appealedAt,
		&appealDecision, &appealResolvedBy, &appealResolvedAt,
		&report.CreatedAt, &report.UpdatedAt, &resolvedAt,
		&report.ReporterName, &report.AssigneeName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	report.AssigneeID = assigneeID
	report.ResolvedBy = resolvedBy
	report.AdminNotes = adminNotes
	report.ResolvedAt = resolvedAt

	if appealStatus != nil {
		as := AppealStatus(*appealStatus)
		report.AppealStatus = &as
	}
	report.AppealReason = appealReason
	report.AppealedAt = appealedAt
	report.AppealDecision = appealDecision
	report.AppealResolvedBy = appealResolvedBy
	report.AppealResolvedAt = appealResolvedAt

	return report, nil
}

func (r *Repository) ListReports(ctx context.Context, filter ReportFilter) ([]*Report, int, error) {
	if filter.Limit <= 0 || filter.Limit > 100 {
		filter.Limit = 50
	}

	where := "1=1"
	args := []interface{}{}
	i := 1

	if filter.Status != nil {
		where += " AND mr.status = $" + itoa(i)
		args = append(args, string(*filter.Status))
		i++
	}
	if filter.TargetType != nil {
		where += " AND mr.target_type = $" + itoa(i)
		args = append(args, *filter.TargetType)
		i++
	}
	if filter.AssigneeID != nil {
		where += " AND mr.assignee_id = $" + itoa(i)
		args = append(args, *filter.AssigneeID)
		i++
	}
	if filter.PriorityGt != nil {
		where += " AND mr.priority >= $" + itoa(i)
		args = append(args, *filter.PriorityGt)
		i++
	}

	// Count
	var total int
	err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM moderation_reports mr WHERE "+where, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Fetch with offset/limit
	args = append(args, filter.Limit, filter.Offset)
	query := `SELECT mr.id, mr.reporter_id, mr.assignee_id, mr.target_type, mr.target_id,
	                 mr.reason, mr.status, mr.priority, mr.created_at, mr.updated_at, mr.resolved_at,
	                 u.display_name AS reporter_name,
	                 COALESCE(a.display_name, '') AS assignee_name
	          FROM moderation_reports mr
	          JOIN users u ON u.id = mr.reporter_id
	          LEFT JOIN users a ON a.id = mr.assignee_id
	          WHERE ` + where + `
	          ORDER BY mr.priority DESC, mr.created_at ASC
	          LIMIT $` + itoa(i) + ` OFFSET $` + itoa(i+1)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var reports []*Report
	for rows.Next() {
		report := &Report{}
		var assigneeID *string
		var resolvedAt *time.Time

		err := rows.Scan(&report.ID, &report.ReporterID, &assigneeID, &report.TargetType, &report.TargetID,
			&report.Reason, &report.Status, &report.Priority, &report.CreatedAt, &report.UpdatedAt, &resolvedAt,
			&report.ReporterName, &report.AssigneeName)
		if err != nil {
			return nil, 0, err
		}
		report.AssigneeID = assigneeID
		report.ResolvedAt = resolvedAt
		reports = append(reports, report)
	}

	return reports, total, nil
}

func (r *Repository) AssignReport(ctx context.Context, reportID, assigneeID string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE moderation_reports SET assignee_id = $1, status = 'assigned', updated_at = NOW()
		 WHERE id = $2 AND (status = 'open' OR status = 'assigned')`,
		assigneeID, reportID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("report not available for assignment")
	}
	return nil
}

func (r *Repository) UnassignReport(ctx context.Context, reportID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE moderation_reports SET assignee_id = NULL, status = 'open', updated_at = NOW()
		 WHERE id = $1 AND status = 'assigned'`, reportID)
	return err
}

func (r *Repository) ResolveReport(ctx context.Context, reportID, resolvedBy string, status ReportStatus) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE moderation_reports SET status = $1, resolved_by = $2, resolved_at = NOW(), updated_at = NOW()
		 WHERE id = $3 AND (status = 'open' OR status = 'assigned')`,
		string(status), resolvedBy, reportID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("report already resolved or dismissed")
	}
	return nil
}

func (r *Repository) SetAdminNotes(ctx context.Context, reportID, notes string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE moderation_reports SET admin_notes = $1, updated_at = NOW() WHERE id = $2`,
		notes, reportID)
	return err
}

func (r *Repository) SubmitAppeal(ctx context.Context, reportID, reason string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE moderation_reports
		 SET appeal_status = 'pending', appeal_reason = $1, appealed_at = NOW(), updated_at = NOW()
		 WHERE id = $2 AND status = 'resolved' AND appeal_status IS NULL`,
		reason, reportID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("report cannot be appealed")
	}
	return nil
}

func (r *Repository) ResolveAppeal(ctx context.Context, reportID, resolvedBy, decision, notes string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE moderation_reports
		 SET appeal_status = $1, appeal_decision = $2, appeal_resolved_by = $3,
		     appeal_resolved_at = NOW(), admin_notes = COALESCE(admin_notes || E'\n' || $4, $4),
		     updated_at = NOW()
		 WHERE id = $5 AND appeal_status = 'pending'`,
		decision, decision, resolvedBy, notes, reportID)
	return err
}

// ─── Moderation Actions ───────────────────────────────────────────────────────

func (r *Repository) CreateAction(ctx context.Context, actorID string, reportID *string, targetType string, targetID string, actionType ModerationActionType, reason *string, metadata map[string]interface{}, adminNotes *string) (*ModerationAction, error) {
	metaJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}

	action := &ModerationAction{}
	err = r.db.QueryRow(ctx,
		`INSERT INTO moderation_actions (actor_id, report_id, target_type, target_id, action_type, reason, metadata, admin_notes)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, actor_id, report_id, target_type, target_id, action_type, reason, metadata, admin_notes, created_at`,
		actorID, reportID, targetType, targetID, string(actionType), reason, metaJSON, adminNotes,
	).Scan(&action.ID, &action.ActorID, &action.ReportID, &action.TargetType, &action.TargetID,
		&action.ActionType, &action.Reason, &action.Metadata, &action.AdminNotes, &action.CreatedAt)
	if err != nil {
		return nil, err
	}
	return action, nil
}

func (r *Repository) GetActionByID(ctx context.Context, actionID string) (*ModerationAction, error) {
	a := &ModerationAction{}
	var reportID, reason, adminNotes, reversedBy, reversedReason *string
	var reversedAt *time.Time
	var metadata []byte
	var createdAt time.Time

	err := r.db.QueryRow(ctx,
		`SELECT ma.id, ma.actor_id, ma.report_id, ma.target_type, ma.target_id,
		        ma.action_type, ma.reason, ma.metadata, ma.admin_notes,
		        ma.reversed_by, ma.reversed_at, ma.reversed_reason, ma.created_at,
		        u.display_name AS actor_name
		 FROM moderation_actions ma
		 JOIN users u ON u.id = ma.actor_id
		 WHERE ma.id = $1`, actionID,
	).Scan(&a.ID, &a.ActorID, &reportID, &a.TargetType, &a.TargetID,
		&a.ActionType, &reason, &metadata, &adminNotes,
		&reversedBy, &reversedAt, &reversedReason, &createdAt, &a.ActorName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	a.ReportID = reportID
	a.Reason = reason
	a.AdminNotes = adminNotes
	a.ReversedBy = reversedBy
	a.ReversedAt = reversedAt
	a.ReversedReason = reversedReason
	a.CreatedAt = createdAt
	if metadata != nil {
		a.Metadata = json.RawMessage(metadata)
	}

	return a, nil
}

func (r *Repository) ListActions(ctx context.Context, filter ActionFilter) ([]*ModerationAction, int, error) {
	if filter.Limit <= 0 || filter.Limit > 100 {
		filter.Limit = 50
	}

	where := "1=1"
	args := []interface{}{}
	i := 1

	if filter.TargetType != nil {
		where += " AND ma.target_type = $" + itoa(i)
		args = append(args, *filter.TargetType)
		i++
	}
	if filter.TargetID != nil {
		where += " AND ma.target_id = $" + itoa(i)
		args = append(args, *filter.TargetID)
		i++
	}
	if filter.ActorID != nil {
		where += " AND ma.actor_id = $" + itoa(i)
		args = append(args, *filter.ActorID)
		i++
	}

	var total int
	err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM moderation_actions ma WHERE "+where, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	args = append(args, filter.Limit, filter.Offset)
	query := `SELECT ma.id, ma.actor_id, ma.report_id, ma.target_type, ma.target_id,
	                 ma.action_type, ma.reason, ma.metadata, ma.admin_notes,
	                 ma.reversed_by, ma.reversed_at, ma.reversed_reason, ma.created_at,
	                 u.display_name AS actor_name
	          FROM moderation_actions ma
	          JOIN users u ON u.id = ma.actor_id
	          WHERE ` + where + `
	          ORDER BY ma.created_at DESC
	          LIMIT $` + itoa(i) + ` OFFSET $` + itoa(i+1)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var actions []*ModerationAction
	for rows.Next() {
		a := &ModerationAction{}
		var reportID, reason, adminNotes, reversedBy, reversedReason *string
		var reversedAt *time.Time
		var metadata []byte

		err := rows.Scan(&a.ID, &a.ActorID, &reportID, &a.TargetType, &a.TargetID,
			&a.ActionType, &reason, &metadata, &adminNotes,
			&reversedBy, &reversedAt, &reversedReason, &a.CreatedAt, &a.ActorName)
		if err != nil {
			return nil, 0, err
		}
		a.ReportID = reportID
		a.Reason = reason
		a.AdminNotes = adminNotes
		a.ReversedBy = reversedBy
		a.ReversedAt = reversedAt
		a.ReversedReason = reversedReason
		if metadata != nil {
			a.Metadata = json.RawMessage(metadata)
		}
		actions = append(actions, a)
	}

	return actions, total, nil
}

func (r *Repository) ReverseAction(ctx context.Context, actionID, reversedBy, reason string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE moderation_actions SET reversed_by = $1, reversed_at = NOW(), reversed_reason = $2
		 WHERE id = $3 AND reversed_at IS NULL`,
		reversedBy, reason, actionID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("action not found or already reversed")
	}
	return nil
}

// ─── Audit Log ────────────────────────────────────────────────────────────────

func (r *Repository) CreateAuditLog(ctx context.Context, actorID *string, action string, targetType, targetID *string, metadata map[string]interface{}, ipAddress, userAgent *string) error {
	metaJSON, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx,
		`INSERT INTO audit_log (actor_id, action, target_type, target_id, metadata, ip_address, user_agent)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		actorID, action, targetType, targetID, metaJSON, ipAddress, userAgent)
	return err
}

func (r *Repository) ListAuditLog(ctx context.Context, filter AuditFilter) ([]*AuditLogEntry, int, error) {
	if filter.Limit <= 0 || filter.Limit > 100 {
		filter.Limit = 50
	}

	where := "1=1"
	args := []interface{}{}
	i := 1

	if filter.ActorID != nil {
		where += " AND al.actor_id = $" + itoa(i)
		args = append(args, *filter.ActorID)
		i++
	}
	if filter.Action != nil {
		where += " AND al.action = $" + itoa(i)
		args = append(args, *filter.Action)
		i++
	}
	if filter.TargetType != nil {
		where += " AND al.target_type = $" + itoa(i)
		args = append(args, *filter.TargetType)
		i++
	}
	if filter.TargetID != nil {
		where += " AND al.target_id = $" + itoa(i)
		args = append(args, *filter.TargetID)
		i++
	}
	if filter.Since != nil {
		where += " AND al.created_at >= $" + itoa(i)
		args = append(args, *filter.Since)
		i++
	}
	if filter.Until != nil {
		where += " AND al.created_at <= $" + itoa(i)
		args = append(args, *filter.Until)
		i++
	}

	var total int
	err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM audit_log al WHERE "+where, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	args = append(args, filter.Limit, filter.Offset)
	query := `SELECT al.id, al.actor_id, al.action, al.target_type, al.target_id,
	                 al.metadata, al.ip_address, al.user_agent, al.created_at,
	                 COALESCE(u.display_name, '') AS actor_name
	          FROM audit_log al
	          LEFT JOIN users u ON u.id = al.actor_id
	          WHERE ` + where + `
	          ORDER BY al.created_at DESC
	          LIMIT $` + itoa(i) + ` OFFSET $` + itoa(i+1)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var entries []*AuditLogEntry
	for rows.Next() {
		e := &AuditLogEntry{}
		var actorID, targetType, targetID, ipAddr, userAgent *string
		var metadata []byte

		err := rows.Scan(&e.ID, &actorID, &e.Action, &targetType, &targetID,
			&metadata, &ipAddr, &userAgent, &e.CreatedAt, &e.ActorName)
		if err != nil {
			return nil, 0, err
		}
		e.ActorID = actorID
		e.TargetType = targetType
		e.TargetID = targetID
		e.IPAddress = ipAddr
		e.UserAgent = userAgent
		if metadata != nil {
			e.Metadata = json.RawMessage(metadata)
		}
		entries = append(entries, e)
	}

	return entries, total, nil
}

// itoa converts a small integer to its decimal string representation.
func itoa(n int) string {
	const digits = "0123456789"
	if n < 10 {
		return string(digits[n])
	}
	b := make([]byte, 0, 3)
	for n > 0 {
		b = append([]byte{digits[n%10]}, b...)
		n /= 10
	}
	return string(b)
}