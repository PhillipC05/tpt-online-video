package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tpt-online-video/services/api/internal/http/middleware"
	"github.com/tpt-online-video/services/api/internal/moderation"
)

type ModerationHandler struct {
	logger  *slog.Logger
	db      *pgxpool.Pool
	svc     *moderation.Service
}

func NewModerationHandler(logger *slog.Logger, db *pgxpool.Pool, modSvc *moderation.Service) *ModerationHandler {
	return &ModerationHandler{
		logger: logger,
		db:     db,
		svc:    modSvc,
	}
}

// ─── Reports ──────────────────────────────────────────────────────────────────

type CreateReportRequest struct {
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	Reason     string `json:"reason"`
}

func (h *ModerationHandler) CreateReport(w http.ResponseWriter, r *http.Request) {
	callerID := middleware.GetUserID(r)
	if callerID == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req CreateReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Reason = strings.TrimSpace(req.Reason)
	if req.Reason == "" {
		writeError(w, http.StatusBadRequest, "reason cannot be empty")
		return
	}
	if len(req.Reason) > 2000 {
		writeError(w, http.StatusBadRequest, "reason too long (max 2000 characters)")
		return
	}

	validTargets := map[string]bool{
		"video": true, "comment": true, "user": true,
		"live_stream": true, "live_chat_message": true,
	}
	if !validTargets[req.TargetType] {
		writeError(w, http.StatusBadRequest, "invalid target type. Valid: video, comment, user, live_stream, live_chat_message")
		return
	}

	report, err := h.svc.CreateReport(r.Context(), callerID, req.TargetType, req.TargetID, req.Reason)
	if err != nil {
		h.logger.Error("create report", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create report")
		return
	}

	writeJSON(w, http.StatusCreated, report)
}

func (h *ModerationHandler) GetReport(w http.ResponseWriter, r *http.Request) {
	reportID := chi.URLParam(r, "reportID")
	if reportID == "" {
		writeError(w, http.StatusBadRequest, "report ID is required")
		return
	}

	report, err := h.svc.GetReport(r.Context(), reportID)
	if err != nil {
		h.logger.Error("get report", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if report == nil {
		writeError(w, http.StatusNotFound, "report not found")
		return
	}

	writeJSON(w, http.StatusOK, report)
}

func (h *ModerationHandler) ListReports(w http.ResponseWriter, r *http.Request) {
	filter := parseReportFilter(r)

	reports, total, err := h.svc.ListReports(r.Context(), filter)
	if err != nil {
		h.logger.Error("list reports", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"reports": reports,
		"total":   total,
		"limit":   filter.Limit,
		"offset":  filter.Offset,
	})
}

func parseReportFilter(r *http.Request) moderation.ReportFilter {
	filter := moderation.ReportFilter{
		Offset: 0,
		Limit:  50,
	}

	if s := r.URL.Query().Get("status"); s != "" {
		status := moderation.ReportStatus(s)
		filter.Status = &status
	}
	if tt := r.URL.Query().Get("target_type"); tt != "" {
		filter.TargetType = &tt
	}
	if a := r.URL.Query().Get("assignee_id"); a != "" {
		filter.AssigneeID = &a
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := parseInt(offsetStr); err == nil && offset >= 0 {
			filter.Offset = offset
		}
	}
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := parseInt(limitStr); err == nil && limit > 0 && limit <= 100 {
			filter.Limit = limit
		}
	}

	return filter
}

// ─── Report Assignment ────────────────────────────────────────────────────────

type AssignReportRequest struct {
	AssigneeID string `json:"assignee_id"`
}

func (h *ModerationHandler) AssignReport(w http.ResponseWriter, r *http.Request) {
	reportID := chi.URLParam(r, "reportID")
	if reportID == "" {
		writeError(w, http.StatusBadRequest, "report ID is required")
		return
	}

	var req AssignReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.AssigneeID == "" {
		writeError(w, http.StatusBadRequest, "assignee_id is required")
		return
	}

	if err := h.svc.AssignReport(r.Context(), reportID, req.AssigneeID); err != nil {
		if err.Error() == "report not available for assignment" {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		h.logger.Error("assign report", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	// Audit log
	actorID := middleware.GetUserID(r)
	h.svc.CreateAuditLog(r.Context(), &actorID, "moderation:assign_report",
		strPtr("report"), &reportID,
		map[string]interface{}{"assignee_id": req.AssigneeID},
		remoteAddrPtr(r), userAgentPtr(r))

	writeJSON(w, http.StatusOK, map[string]string{"status": "assigned"})
}

func (h *ModerationHandler) UnassignReport(w http.ResponseWriter, r *http.Request) {
	reportID := chi.URLParam(r, "reportID")
	if reportID == "" {
		writeError(w, http.StatusBadRequest, "report ID is required")
		return
	}

	if err := h.svc.UnassignReport(r.Context(), reportID); err != nil {
		h.logger.Error("unassign report", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "unassigned"})
}

// ─── Report Resolution ────────────────────────────────────────────────────────

type ResolveReportRequest struct {
	ActionType  string `json:"action_type,omitempty"`
	Reason      string `json:"reason,omitempty"`
	ActionNotes string `json:"admin_notes,omitempty"`
}

func (h *ModerationHandler) ResolveReport(w http.ResponseWriter, r *http.Request) {
	reportID := chi.URLParam(r, "reportID")
	callerID := middleware.GetUserID(r)
	if reportID == "" {
		writeError(w, http.StatusBadRequest, "report ID is required")
		return
	}

	var req ResolveReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	validationMessage := resolveRequestValidation(req)

	if validationMessage != "" {
		writeError(w, http.StatusBadRequest, validationMessage)
		return
	}
	// Resolve the report
	if err := h.svc.ResolveReport(r.Context(), reportID, callerID); err != nil {
		h.logger.Error("resolve report", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	// If an action type is specified, take moderation action
	if req.ActionType != "" {
		actionType := moderation.ModerationActionType(req.ActionType)

		// Get the report to find target info
		report, err := h.svc.GetReport(r.Context(), reportID)
		if err == nil && report != nil {
			reasonStr := req.Reason
			notesStr := req.ActionNotes
			_, err := h.svc.ExecuteAction(r.Context(), callerID, &reportID,
				report.TargetType, report.TargetID,
				actionType, &reasonStr,
				map[string]interface{}{"report_id": reportID},
				&notesStr, remoteAddrPtr(r), userAgentPtr(r))
			if err != nil {
				h.logger.Error("execute action on resolve", "error", err)
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "resolved"})
}

func resolveRequestValidation(req ResolveReportRequest) string {
	validationMessage := ""
	if req.ActionType != "" {
		validActions := map[string]bool{
			"hide_content": true, "unpublish_video": true, "delete_video": true,
			"remove_comment": true, "terminate_live_stream": true, "lock_live_chat": true,
			"suspend_user": true, "ban_user": true, "restore_content": true,
		}
		if !validActions[req.ActionType] {
			validationMessage = "invalid action type"
		}
	}
	return validationMessage
}

func (h *ModerationHandler) DismissReport(w http.ResponseWriter, r *http.Request) {
	reportID := chi.URLParam(r, "reportID")
	callerID := middleware.GetUserID(r)
	if reportID == "" {
		writeError(w, http.StatusBadRequest, "report ID is required")
		return
	}

	if err := h.svc.DismissReport(r.Context(), reportID, callerID); err != nil {
		h.logger.Error("dismiss report", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "dismissed"})
}

// ─── Admin Notes ──────────────────────────────────────────────────────────────

type AdminNotesRequest struct {
	Notes string `json:"notes"`
}

func (h *ModerationHandler) SetAdminNotes(w http.ResponseWriter, r *http.Request) {
	reportID := chi.URLParam(r, "reportID")
	if reportID == "" {
		writeError(w, http.StatusBadRequest, "report ID is required")
		return
	}

	var req AdminNotesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.SetAdminNotes(r.Context(), reportID, req.Notes); err != nil {
		h.logger.Error("set admin notes", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// ─── Appeals ──────────────────────────────────────────────────────────────────

type SubmitAppealRequest struct {
	Reason string `json:"reason"`
}

func (h *ModerationHandler) SubmitAppeal(w http.ResponseWriter, r *http.Request) {
	reportID := chi.URLParam(r, "reportID")
	if reportID == "" {
		writeError(w, http.StatusBadRequest, "report ID is required")
		return
	}

	var req SubmitAppealRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if req.Reason == "" {
		writeError(w, http.StatusBadRequest, "reason cannot be empty")
		return
	}
	if len(req.Reason) > 2000 {
		writeError(w, http.StatusBadRequest, "reason too long (max 2000 characters)")
		return
	}

	if err := h.svc.SubmitAppeal(r.Context(), reportID, req.Reason); err != nil {
		if err.Error() == "report cannot be appealed" {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		h.logger.Error("submit appeal", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "appeal submitted"})
}

type ResolveAppealRequest struct {
	Decision string `json:"decision"` // "granted" or "denied"
	Notes    string `json:"notes,omitempty"`
}

func (h *ModerationHandler) ResolveAppeal(w http.ResponseWriter, r *http.Request) {
	reportID := chi.URLParam(r, "reportID")
	callerID := middleware.GetUserID(r)
	if reportID == "" {
		writeError(w, http.StatusBadRequest, "report ID is required")
		return
	}

	var req ResolveAppealRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Decision != "granted" && req.Decision != "denied" {
		writeError(w, http.StatusBadRequest, "decision must be 'granted' or 'denied'")
		return
	}

	if err := h.svc.ResolveAppeal(r.Context(), reportID, callerID, req.Decision, req.Notes); err != nil {
		h.logger.Error("resolve appeal", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "appeal " + req.Decision})
}

// ─── Moderation Actions ───────────────────────────────────────────────────────

type ExecuteActionRequest struct {
	ReportID   *string                `json:"report_id,omitempty"`
	TargetType string                 `json:"target_type"`
	TargetID   string                 `json:"target_id"`
	ActionType string                 `json:"action_type"`
	Reason     string                 `json:"reason,omitempty"`
	AdminNotes string                 `json:"admin_notes,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

func (h *ModerationHandler) ExecuteAction(w http.ResponseWriter, r *http.Request) {
	callerID := middleware.GetUserID(r)
	if callerID == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req ExecuteActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.TargetType == "" || req.TargetID == "" || req.ActionType == "" {
		writeError(w, http.StatusBadRequest, "target_type, target_id, and action_type are required")
		return
	}

	actionType := moderation.ModerationActionType(req.ActionType)
	validActions := map[string]bool{
		"hide_content": true, "unpublish_video": true, "delete_video": true,
		"remove_comment": true, "terminate_live_stream": true, "lock_live_chat": true,
		"suspend_user": true, "ban_user": true, "restore_content": true,
	}
	if !validActions[req.ActionType] {
		writeError(w, http.StatusBadRequest, "invalid action type")
		return
	}

	reasonStr := req.Reason
	notesStr := req.AdminNotes
	if req.Metadata == nil {
		req.Metadata = make(map[string]interface{})
	}

	result, err := h.svc.ExecuteAction(r.Context(), callerID, req.ReportID,
		req.TargetType, req.TargetID, actionType,
		&reasonStr, req.Metadata, &notesStr,
		remoteAddrPtr(r), userAgentPtr(r))
	if err != nil {
		h.logger.Error("execute action", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to execute action")
		return
	}

	writeJSON(w, http.StatusCreated, result.Action)
}

func (h *ModerationHandler) ListActions(w http.ResponseWriter, r *http.Request) {
	filter := parseActionFilter(r)

	actions, total, err := h.svc.ListActions(r.Context(), filter)
	if err != nil {
		h.logger.Error("list actions", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"actions": actions,
		"total":   total,
		"limit":   filter.Limit,
		"offset":  filter.Offset,
	})
}

func (h *ModerationHandler) GetAction(w http.ResponseWriter, r *http.Request) {
	actionID := chi.URLParam(r, "actionID")
	if actionID == "" {
		writeError(w, http.StatusBadRequest, "action ID is required")
		return
	}

	action, err := h.svc.GetAction(r.Context(), actionID)
	if err != nil {
		h.logger.Error("get action", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if action == nil {
		writeError(w, http.StatusNotFound, "action not found")
		return
	}

	writeJSON(w, http.StatusOK, action)
}

type ReverseActionRequest struct {
	Reason string `json:"reason"`
}

func (h *ModerationHandler) ReverseAction(w http.ResponseWriter, r *http.Request) {
	actionID := chi.URLParam(r, "actionID")
	callerID := middleware.GetUserID(r)
	if actionID == "" {
		writeError(w, http.StatusBadRequest, "action ID is required")
		return
	}

	var req ReverseActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if req.Reason == "" {
		writeError(w, http.StatusBadRequest, "reason is required to reverse an action")
		return
	}

	if err := h.svc.ReverseAction(r.Context(), actionID, &callerID, &req.Reason, remoteAddrPtr(r), userAgentPtr(r)); err != nil {
		if err.Error() == "action not found" {
			writeError(w, http.StatusNotFound, "action not found")
			return
		}
		if err.Error() == "action already reversed" {
			writeError(w, http.StatusConflict, "action already reversed")
			return
		}
		h.logger.Error("reverse action", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to reverse action")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "reversed"})
}

func parseActionFilter(r *http.Request) moderation.ActionFilter {
	filter := moderation.ActionFilter{
		Offset: 0,
		Limit:  50,
	}

	if tt := r.URL.Query().Get("target_type"); tt != "" {
		filter.TargetType = &tt
	}
	if tid := r.URL.Query().Get("target_id"); tid != "" {
		filter.TargetID = &tid
	}
	if aid := r.URL.Query().Get("actor_id"); aid != "" {
		filter.ActorID = &aid
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := parseInt(offsetStr); err == nil && offset >= 0 {
			filter.Offset = offset
		}
	}
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := parseInt(limitStr); err == nil && limit > 0 && limit <= 100 {
			filter.Limit = limit
		}
	}

	return filter
}

// ─── Audit Log ────────────────────────────────────────────────────────────────

func (h *ModerationHandler) ListAuditLog(w http.ResponseWriter, r *http.Request) {
	filter := parseAuditFilter(r)

	entries, total, err := h.svc.ListAuditLog(r.Context(), filter)
	if err != nil {
		h.logger.Error("list audit log", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"entries": entries,
		"total":   total,
		"limit":   filter.Limit,
		"offset":  filter.Offset,
	})
}

func parseAuditFilter(r *http.Request) moderation.AuditFilter {
	filter := moderation.AuditFilter{
		Offset: 0,
		Limit:  50,
	}

	if aid := r.URL.Query().Get("actor_id"); aid != "" {
		filter.ActorID = &aid
	}
	if action := r.URL.Query().Get("action"); action != "" {
		filter.Action = &action
	}
	if tt := r.URL.Query().Get("target_type"); tt != "" {
		filter.TargetType = &tt
	}
	if tid := r.URL.Query().Get("target_id"); tid != "" {
		filter.TargetID = &tid
	}
	if s := r.URL.Query().Get("since"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			filter.Since = &t
		}
	}
	if u := r.URL.Query().Get("until"); u != "" {
		if t, err := time.Parse(time.RFC3339, u); err == nil {
			filter.Until = &t
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := parseInt(offsetStr); err == nil && offset >= 0 {
			filter.Offset = offset
		}
	}
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := parseInt(limitStr); err == nil && limit > 0 && limit <= 100 {
			filter.Limit = limit
		}
	}

	return filter
}

// ─── Dashboard Stats ──────────────────────────────────────────────────────────

func (h *ModerationHandler) DashboardStats(w http.ResponseWriter, r *http.Request) {
	var openCount, assignedCount, resolvedCount, dismissedCount, pendingAppeals int

	h.db.QueryRow(r.Context(),
		`SELECT COUNT(*) FROM moderation_reports WHERE status = 'open'`).Scan(&openCount)
	h.db.QueryRow(r.Context(),
		`SELECT COUNT(*) FROM moderation_reports WHERE status = 'assigned'`).Scan(&assignedCount)
	h.db.QueryRow(r.Context(),
		`SELECT COUNT(*) FROM moderation_reports WHERE status = 'resolved'`).Scan(&resolvedCount)
	h.db.QueryRow(r.Context(),
		`SELECT COUNT(*) FROM moderation_reports WHERE status = 'dismissed'`).Scan(&dismissedCount)
	h.db.QueryRow(r.Context(),
		`SELECT COUNT(*) FROM moderation_reports WHERE appeal_status = 'pending'`).Scan(&pendingAppeals)

	var actionsToday int
	h.db.QueryRow(r.Context(),
		`SELECT COUNT(*) FROM moderation_actions WHERE created_at >= NOW() - INTERVAL '24 hours'`).Scan(&actionsToday)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"open_reports":       openCount,
		"assigned_reports":   assignedCount,
		"resolved_reports":   resolvedCount,
		"dismissed_reports":  dismissedCount,
		"pending_appeals":    pendingAppeals,
		"actions_last_24h":   actionsToday,
	})
}

// ─── Helper types for creator reports ─────────────────────────────────────────

type CreateVideoReportRequest struct {
	Reason string `json:"reason"`
}

func (h *ModerationHandler) ReportVideo(w http.ResponseWriter, r *http.Request) {
	videoID := chi.URLParam(r, "videoID")
	callerID := middleware.GetUserID(r)
	if callerID == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req CreateVideoReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if req.Reason == "" {
		writeError(w, http.StatusBadRequest, "reason cannot be empty")
		return
	}

	report, err := h.svc.CreateReport(r.Context(), callerID, "video", videoID, req.Reason)
	if err != nil {
		h.logger.Error("report video", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create report")
		return
	}

	writeJSON(w, http.StatusCreated, report)
}

type CreateUserReportRequest struct {
	Reason string `json:"reason"`
}

func (h *ModerationHandler) ReportUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	callerID := middleware.GetUserID(r)
	if callerID == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req CreateUserReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if req.Reason == "" {
		writeError(w, http.StatusBadRequest, "reason cannot be empty")
		return
	}

	report, err := h.svc.CreateReport(r.Context(), callerID, "user", userID, req.Reason)
	if err != nil {
		h.logger.Error("report user", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create report")
		return
	}

	writeJSON(w, http.StatusCreated, report)
}

type CreateLiveStreamReportRequest struct {
	Reason string `json:"reason"`
}

func (h *ModerationHandler) ReportLiveStream(w http.ResponseWriter, r *http.Request) {
	streamID := chi.URLParam(r, "streamID")
	callerID := middleware.GetUserID(r)
	if callerID == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req CreateLiveStreamReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if req.Reason == "" {
		writeError(w, http.StatusBadRequest, "reason cannot be empty")
		return
	}

	report, err := h.svc.CreateReport(r.Context(), callerID, "live_stream", streamID, req.Reason)
	if err != nil {
		h.logger.Error("report live stream", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create report")
		return
	}

	writeJSON(w, http.StatusCreated, report)
}

type CreateLiveChatReportRequest struct {
	Reason string `json:"reason"`
}

func (h *ModerationHandler) ReportLiveChatMessage(w http.ResponseWriter, r *http.Request) {
	messageID := chi.URLParam(r, "messageID")
	callerID := middleware.GetUserID(r)
	if callerID == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req CreateLiveChatReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if req.Reason == "" {
		writeError(w, http.StatusBadRequest, "reason cannot be empty")
		return
	}

	report, err := h.svc.CreateReport(r.Context(), callerID, "live_chat_message", messageID, req.Reason)
	if err != nil {
		h.logger.Error("report live chat message", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create report")
		return
	}

	writeJSON(w, http.StatusCreated, report)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func parseInt(s string) (int, error) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, errors.New("not a number")
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

func strPtr(s string) *string {
	return &s
}

func remoteAddrPtr(r *http.Request) *string {
	addr := r.RemoteAddr
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		addr = strings.SplitN(xff, ",", 2)[0]
	}
	return &addr
}

func userAgentPtr(r *http.Request) *string {
	ua := r.UserAgent()
	return &ua
}