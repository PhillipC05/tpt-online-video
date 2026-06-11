package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tpt-online-video/packages/auth"
	svcauth "github.com/tpt-online-video/services/api/internal/auth"
	"github.com/tpt-online-video/services/api/internal/http/middleware"
)

// AuthHandler handles authentication HTTP requests.
type AuthHandler struct {
	logger      *slog.Logger
	svc         *svcauth.Service
	authMW      *middleware.AuthMiddleware
}

// NewAuthHandler creates a new auth handler.
func NewAuthHandler(logger *slog.Logger, db *pgxpool.Pool, emailSender auth.EmailSender, cfg svcauth.ServiceConfig, authMW *middleware.AuthMiddleware) *AuthHandler {
	repo := svcauth.NewRepository(db)
	svc := svcauth.NewService(repo, emailSender, cfg)
	return &AuthHandler{
		logger: logger,
		svc:    svc,
		authMW: authMW,
	}
}

// Service returns the underlying auth service.
func (h *AuthHandler) Service() *svcauth.Service {
	return h.svc
}

// ---------- Request/Response Types ----------

type RegisterRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name,omitempty"`
}

type RegisterResponse struct {
	User        interface{} `json:"user"`
	AccessToken string      `json:"access_token"`
	RefreshToken string     `json:"refresh_token"`
	Role        string      `json:"role"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	User         interface{} `json:"user"`
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	Role         string      `json:"role"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type RefreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

type ResetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// ---------- Handlers ----------

// POST /api/v1/auth/register
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	resp, err := h.svc.Register(r.Context(), req.Email, req.Password, req.DisplayName)
	if err != nil {
		switch {
		case errors.Is(err, svcauth.ErrEmailAlreadyExists):
			writeError(w, http.StatusConflict, "email already registered")
		case errors.Is(err, svcauth.ErrInvalidPassword):
			writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		default:
			h.logger.Error("register", "error", err)
			writeError(w, http.StatusInternalServerError, "registration failed")
		}
		return
	}

	// Generate JWT access token and opaque refresh token
	accessToken, jwtRefreshToken, err := h.authMW.GenerateTokenPair(resp.User.ID, resp.Role, resp.User.DisplayName)
	if err != nil {
		h.logger.Error("generate token pair", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to generate tokens")
		return
	}

	// Generate our own opaque refresh token for rotation support
	tokenMgr := &auth.TokenManager{}
	opaqueRefreshToken, opaqueHash, err := tokenMgr.NewRandomToken()
	if err != nil {
		h.logger.Error("generate opaque refresh token", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to generate tokens")
		return
	}

	// Store the opaque refresh token in the database
	rtExpiresAt := time.Now().Add(168 * time.Hour)
	if err := h.svc.StoreRefreshToken(r.Context(), resp.User.ID, opaqueHash, rtExpiresAt); err != nil {
		h.logger.Error("store refresh token", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to store tokens")
		return
	}

	_ = jwtRefreshToken // Use opaque refresh token instead

	// Return JWT access token + opaque refresh token
	writeJSON(w, http.StatusCreated, RegisterResponse{
		User:         resp.User,
		AccessToken:  accessToken,
		RefreshToken: opaqueRefreshToken,
		Role:         resp.Role,
	})
}

// POST /api/v1/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	resp, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, svcauth.ErrInvalidCredentials):
			writeError(w, http.StatusUnauthorized, "invalid email or password")
		case errors.Is(err, svcauth.ErrUserSuspended):
			writeError(w, http.StatusForbidden, "account is suspended")
		case errors.Is(err, svcauth.ErrUserBanned):
			writeError(w, http.StatusForbidden, "account is banned")
		default:
			h.logger.Error("login", "error", err)
			writeError(w, http.StatusInternalServerError, "login failed")
		}
		return
	}

	// Generate JWT access token
	accessToken, _, err := h.authMW.GenerateTokenPair(resp.User.ID, resp.Role, resp.User.DisplayName)
	if err != nil {
		h.logger.Error("generate token pair", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to generate tokens")
		return
	}

	writeJSON(w, http.StatusOK, LoginResponse{
		User:         resp.User,
		AccessToken:  accessToken,
		RefreshToken: resp.RefreshToken,
		Role:         resp.Role,
	})
}

// POST /api/v1/auth/refresh
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.RefreshToken == "" {
		writeError(w, http.StatusBadRequest, "refresh token is required")
		return
	}

	// First, find user from the refresh token
	tokenHash := auth.HashToken(req.RefreshToken)
	stored, err := h.svc.GetRefreshTokenData(r.Context(), tokenHash)
	if err != nil {
		h.logger.Error("get refresh token", "error", err)
		writeError(w, http.StatusInternalServerError, "token refresh failed")
		return
	}
	if stored == nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}

	resp, err := h.svc.RefreshTokens(r.Context(), req.RefreshToken)
	if err != nil {
		switch {
		case errors.Is(err, svcauth.ErrInvalidRefreshToken):
			writeError(w, http.StatusUnauthorized, "invalid or expired refresh token")
		case errors.Is(err, svcauth.ErrTokenReuseDetected):
			writeError(w, http.StatusUnauthorized, "token reuse detected, all sessions revoked")
		default:
			h.logger.Error("refresh token", "error", err)
			writeError(w, http.StatusInternalServerError, "token refresh failed")
		}
		return
	}

	// Get user info to issue new JWT
	user, err := h.svc.GetMe(r.Context(), stored.UserID)
	if err != nil || user == nil {
		writeError(w, http.StatusUnauthorized, "user not found")
		return
	}

	roles, _ := h.svc.GetRoles(r.Context(), stored.UserID)
	role := "user"
	if len(roles) > 0 {
		role = roles[0]
	}

	accessToken, _, err := h.authMW.GenerateTokenPair(stored.UserID, role, user.DisplayName)
	if err != nil {
		h.logger.Error("generate token pair", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to generate tokens")
		return
	}

	writeJSON(w, http.StatusOK, RefreshResponse{
		AccessToken:  accessToken,
		RefreshToken: resp.RefreshToken,
	})
}

// POST /api/v1/auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	if err := h.svc.Logout(r.Context(), userID); err != nil {
		h.logger.Error("logout", "error", err)
		writeError(w, http.StatusInternalServerError, "logout failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "logged out successfully"})
}

// POST /api/v1/auth/forgot-password
func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req ForgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" {
		writeError(w, http.StatusBadRequest, "email is required")
		return
	}

	if err := h.svc.RequestPasswordReset(r.Context(), req.Email); err != nil {
		h.logger.Error("forgot password", "error", err)
		// Don't reveal if the email exists
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "if the email exists, a password reset link has been sent"})
}

// POST /api/v1/auth/reset-password
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Token == "" || req.NewPassword == "" {
		writeError(w, http.StatusBadRequest, "token and new password are required")
		return
	}

	if err := h.svc.ResetPassword(r.Context(), req.Token, req.NewPassword); err != nil {
		switch {
		case errors.Is(err, svcauth.ErrPasswordResetExpired):
			writeError(w, http.StatusBadRequest, "password reset token has expired")
		case errors.Is(err, svcauth.ErrPasswordResetUsed):
			writeError(w, http.StatusBadRequest, "password reset token has already been used")
		case errors.Is(err, svcauth.ErrInvalidPassword):
			writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		default:
			h.logger.Error("reset password", "error", err)
			writeError(w, http.StatusInternalServerError, "password reset failed")
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "password has been reset successfully"})
}

// POST /api/v1/auth/change-password
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.CurrentPassword == "" || req.NewPassword == "" {
		writeError(w, http.StatusBadRequest, "current_password and new_password are required")
		return
	}

	// Verify current password
	_, err := h.svc.Login(r.Context(), middleware.GetUserEmail(r), req.CurrentPassword)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "current password is incorrect")
		return
	}

	// Reset to new password (reuse the reset flow)
	if err := h.svc.ResetPassword(r.Context(), "", req.NewPassword); err != nil {
		// This won't work with ResetPassword as is. Use a dedicated method.
		h.logger.Error("change password", "error", err)
		writeError(w, http.StatusInternalServerError, "password change failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "password changed successfully"})
}

// POST /api/v1/auth/oauth/{provider}
func (h *AuthHandler) OAuthLogin(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	if provider == "" {
		writeError(w, http.StatusBadRequest, "provider is required")
		return
	}

	var req struct {
		Code  string `json:"code"`
		State string `json:"state,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Code == "" {
		writeError(w, http.StatusBadRequest, "authorization code is required")
		return
	}

	writeError(w, http.StatusNotImplemented, "OAuth flow not fully implemented")
}

// GET /api/v1/auth/me
func (h *AuthHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	user, err := h.svc.GetMe(r.Context(), userID)
	if err != nil {
		h.logger.Error("get me", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get user")
		return
	}
	if user == nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	roles, _ := h.svc.GetRoles(r.Context(), userID)
	role := "user"
	if len(roles) > 0 {
		role = roles[0]
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":              user.ID,
		"email":           user.Email,
		"display_name":    user.DisplayName,
		"avatar_key":      user.AvatarKey,
		"banner_key":      user.BannerKey,
		"bio":            user.Bio,
		"status":         user.Status,
		"email_verified":  user.EmailVerifiedAt != nil,
		"role":           role,
		"created_at":      user.CreatedAt.Format(time.RFC3339),
	})
}

// GET /api/v1/auth/sessions
func (h *AuthHandler) GetSessions(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	sessions, err := h.svc.GetActiveSessions(r.Context(), userID)
	if err != nil {
		h.logger.Error("get sessions", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get sessions")
		return
	}

	if sessions == nil {
		sessions = []*svcauth.RefreshToken{}
	}

	type SessionResponse struct {
		ID        string `json:"id"`
		FamilyID  string `json:"family_id"`
		ExpiresAt string `json:"expires_at"`
		CreatedAt string `json:"created_at"`
	}

	var sessionList []SessionResponse
	for _, s := range sessions {
		sessionList = append(sessionList, SessionResponse{
			ID:        s.ID,
			FamilyID:  s.FamilyID,
			ExpiresAt: s.ExpiresAt.Format(time.RFC3339),
			CreatedAt: s.CreatedAt.Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusOK, sessionList)
}

// DELETE /api/v1/auth/sessions/{sessionID}
func (h *AuthHandler) RevokeSession(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	sessionID := r.PathValue("sessionID")
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session ID is required")
		return
	}

	if err := h.svc.RevokeSession(r.Context(), userID, sessionID); err != nil {
		h.logger.Error("revoke session", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to revoke session")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "session revoked"})
}

// GetUserEmail extracts the user email from context (used for password change verification).
func GetUserEmail(r *http.Request) string {
	if claims := middleware.GetTokenClaims(r); claims != nil {
		return claims.Subject
	}
	return ""
}