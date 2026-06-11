package middleware

import (
	"context"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Context keys for storing auth information.
type contextKey string

const (
	userIDKey    contextKey = "user_id"
	userRoleKey  contextKey = "user_role"
	userNameKey  contextKey = "user_name"
	tokenClaimsKey contextKey = "token_claims"
)

// UserClaims represents the JWT claims for a user.
type UserClaims struct {
	UserID      string   `json:"user_id"`
	Role        string   `json:"role"`
	DisplayName string   `json:"display_name,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
	jwt.RegisteredClaims
}

// AuthConfig configures the auth middleware.
type AuthConfig struct {
	JWTSecret     string
	JWTAccessTTL  time.Duration
	JWTRefreshTTL time.Duration
}

// AuthMiddleware implements JWT-based authentication.
type AuthMiddleware struct {
	config    AuthConfig
	logger    *slog.Logger
	publicPaths []string // paths that don't require authentication
}

// NewAuthMiddleware creates a new auth middleware.
func NewAuthMiddleware(config AuthConfig, logger *slog.Logger, publicPaths ...string) *AuthMiddleware {
	if publicPaths == nil {
		publicPaths = []string{}
	}
	return &AuthMiddleware{
		config:      config,
		logger:      logger,
		publicPaths: publicPaths,
	}
}

// Middleware returns the HTTP middleware handler that authenticates requests.
func (am *AuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if the path is public
		for _, path := range am.publicPaths {
			if strings.HasPrefix(r.URL.Path, path) {
				next.ServeHTTP(w, r)
				return
			}
		}

		// Extract the token from the Authorization header
		token, err := am.extractToken(r)
		if err != nil {
			WriteUnauthorized(w, err.Error())
			return
		}

		// Parse and validate the JWT
		claims, err := am.validateToken(token)
		if err != nil {
			WriteUnauthorized(w, "invalid or expired token")
			return
		}

		// Store claims in context
		ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
		ctx = context.WithValue(ctx, userRoleKey, claims.Role)
		ctx = context.WithValue(ctx, userNameKey, claims.DisplayName)
		ctx = context.WithValue(ctx, tokenClaimsKey, claims)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// OptionalAuthMiddleware is a middleware that authenticates if a token is present,
// but does not require it. Useful for endpoints that work differently for authenticated vs anonymous users.
func (am *AuthMiddleware) OptionalAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := am.extractToken(r)
		if err != nil {
			// No token present, continue without auth
			next.ServeHTTP(w, r)
			return
		}

		claims, err := am.validateToken(token)
		if err != nil {
			// Invalid token, continue without auth
			next.ServeHTTP(w, r)
			return
		}

		ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
		ctx = context.WithValue(ctx, userRoleKey, claims.Role)
		ctx = context.WithValue(ctx, userNameKey, claims.DisplayName)
		ctx = context.WithValue(ctx, tokenClaimsKey, claims)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GenerateTokenPair generates both an access token and a refresh token.
func (am *AuthMiddleware) GenerateTokenPair(userID, role, displayName string) (accessToken, refreshToken string, err error) {
	now := time.Now()

	// Access token
	accessClaims := &UserClaims{
		UserID:      userID,
		Role:        role,
		DisplayName: displayName,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(am.config.JWTAccessTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "tpt-api",
			Subject:   userID,
			ID:        uuid.New().String(),
		},
	}

	accessToken, err = jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString([]byte(am.config.JWTSecret))
	if err != nil {
		return "", "", fmt.Errorf("sign access token: %w", err)
	}

	// Refresh token
	refreshClaims := &jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(now.Add(am.config.JWTRefreshTTL)),
		IssuedAt:  jwt.NewNumericDate(now),
		Issuer:    "tpt-api",
		Subject:   userID,
		ID:        uuid.New().String(),
	}

	refreshToken, err = jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).SignedString([]byte(am.config.JWTSecret))
	if err != nil {
		return "", "", fmt.Errorf("sign refresh token: %w", err)
	}

	return accessToken, refreshToken, nil
}

// ValidateToken validates a JWT token string and returns the claims.
func (am *AuthMiddleware) ValidateToken(tokenString string) (*UserClaims, error) {
	return am.validateToken(tokenString)
}

// extractToken extracts the JWT token from the Authorization header.
func (am *AuthMiddleware) extractToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		// Also check cookie
		cookie, err := r.Cookie("access_token")
		if err == nil && cookie.Value != "" {
			return cookie.Value, nil
		}
		return "", fmt.Errorf("missing authorization header")
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", fmt.Errorf("invalid authorization header format, expected 'Bearer <token>'")
	}

	return parts[1], nil
}

// validateToken parses and validates a JWT token.
func (am *AuthMiddleware) validateToken(tokenString string) (*UserClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &UserClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(am.config.JWTSecret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	claims, ok := token.Claims.(*UserClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}

// GetUserID extracts the user ID from the request context.
func GetUserID(r *http.Request) string {
	if userID, ok := r.Context().Value(userIDKey).(string); ok {
		return userID
	}
	return ""
}

// GetUserRole extracts the user role from the request context.
func GetUserRole(r *http.Request) string {
	if role, ok := r.Context().Value(userRoleKey).(string); ok {
		return role
	}
	return ""
}

// GetUserName extracts the user display name from the request context.
func GetUserName(r *http.Request) string {
	if name, ok := r.Context().Value(userNameKey).(string); ok {
		return name
	}
	return ""
}

// GetTokenClaims extracts the full token claims from the request context.
func GetTokenClaims(r *http.Request) *UserClaims {
	if claims, ok := r.Context().Value(tokenClaimsKey).(*UserClaims); ok {
		return claims
	}
	return nil
}

// APIKeyAuth is a middleware that authenticates using an API key (for service-to-service auth).
type APIKeyAuth struct {
	apiKeys   map[string]string // key -> name mapping
	logger    *slog.Logger
}

// NewAPIKeyAuth creates a new API key auth middleware.
func NewAPIKeyAuth(logger *slog.Logger, apiKeys map[string]string) *APIKeyAuth {
	return &APIKeyAuth{
		apiKeys:   apiKeys,
		logger:    logger,
	}
}

// Middleware returns the HTTP middleware handler.
func (a *APIKeyAuth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("X-API-Key")
		if key == "" {
			WriteUnauthorized(w, "missing API key")
			return
		}

		name, ok := a.apiKeys[key]
		if !ok {
			// Use constant-time comparison to prevent timing attacks
			for storedKey, storedName := range a.apiKeys {
				if subtle.ConstantTimeCompare([]byte(key), []byte(storedKey)) == 1 {
					name = storedName
					ok = true
					break
				}
			}
		}

		if !ok {
			WriteUnauthorized(w, "invalid API key")
			return
		}

		ctx := context.WithValue(r.Context(), userIDKey, "service:"+name)
		ctx = context.WithValue(ctx, userRoleKey, "service")
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}