package auth

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tpt-online-video/packages/auth"
)

// User represents a user record from the database.
type User struct {
	ID              string     `json:"id"`
	Email           string     `json:"email"`
	PasswordHash    *string    `json:"-"` // nil for OAuth-only accounts
	DisplayName     string     `json:"display_name"`
	AvatarKey       *string    `json:"avatar_key,omitempty"`
	BannerKey       *string    `json:"banner_key,omitempty"`
	Bio             *string    `json:"bio,omitempty"`
	Status          string     `json:"status"`
	EmailVerifiedAt *time.Time `json:"email_verified_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	DeletedAt       *time.Time `json:"deleted_at,omitempty"`
}

// RefreshToken represents a refresh token record.
type RefreshToken struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	TokenHash string     `json:"-"`
	FamilyID  string     `json:"family_id"`
	ExpiresAt time.Time  `json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// PasswordResetToken represents a password reset token record.
type PasswordResetToken struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	TokenHash string     `json:"-"`
	ExpiresAt time.Time  `json:"expires_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// OAuthAccount represents a linked OAuth account.
type OAuthAccount struct {
	ID                  string     `json:"id"`
	UserID              string     `json:"user_id"`
	Provider            string     `json:"provider"`
	ProviderAccountID   string     `json:"provider_account_id"`
	Email               *string    `json:"email,omitempty"`
	AccessTokenEnc      *string    `json:"-"`
	RefreshTokenEnc     *string    `json:"-"`
	TokenExpiresAt      *time.Time `json:"token_expires_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// Repository handles database operations for auth.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new auth repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// DB returns the underlying database pool.
func (r *Repository) DB() *pgxpool.Pool {
	return r.db
}

// ---------- Users ----------

func (r *Repository) CreateUser(ctx context.Context, email, passwordHash, displayName string) (*User, error) {
	user := &User{}
	err := r.db.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, display_name)
		 VALUES ($1, $2, $3)
		 RETURNING id, email, password_hash, display_name, status, created_at, updated_at`,
		email, passwordHash, displayName,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.DisplayName,
		&user.Status, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	user := &User{}
	err := r.db.QueryRow(ctx,
		`SELECT id, email, password_hash, display_name, avatar_key, banner_key, bio,
		        status, email_verified_at, created_at, updated_at, deleted_at
		 FROM users WHERE email = $1 AND deleted_at IS NULL`,
		email,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.DisplayName,
		&user.AvatarKey, &user.BannerKey, &user.Bio,
		&user.Status, &user.EmailVerifiedAt, &user.CreatedAt, &user.UpdatedAt, &user.DeletedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return user, nil
}

func (r *Repository) GetUserByID(ctx context.Context, id string) (*User, error) {
	user := &User{}
	err := r.db.QueryRow(ctx,
		`SELECT id, email, password_hash, display_name, avatar_key, banner_key, bio,
		        status, email_verified_at, created_at, updated_at, deleted_at
		 FROM users WHERE id = $1 AND deleted_at IS NULL`,
		id,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.DisplayName,
		&user.AvatarKey, &user.BannerKey, &user.Bio,
		&user.Status, &user.EmailVerifiedAt, &user.CreatedAt, &user.UpdatedAt, &user.DeletedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return user, nil
}

func (r *Repository) UpdateUserPassword(ctx context.Context, userID, passwordHash string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE users SET password_hash = $1 WHERE id = $2`,
		passwordHash, userID,
	)
	return err
}

func (r *Repository) VerifyUserEmail(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE users SET email_verified_at = now() WHERE id = $1`,
		userID,
	)
	return err
}

func (r *Repository) UpdateDisplayName(ctx context.Context, userID, displayName string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE users SET display_name = $1 WHERE id = $2`,
		displayName, userID,
	)
	return err
}

// ---------- User Roles ----------

func (r *Repository) GetUserRoles(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.db.Query(ctx,
		`SELECT r.name FROM user_roles ur
		 JOIN roles r ON r.id = ur.role_id
		 WHERE ur.user_id = $1`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []string
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, nil
}

func (r *Repository) AssignDefaultRole(ctx context.Context, userID string) error {
	// Assign the default 'user' role
	_, err := r.db.Exec(ctx,
		`INSERT INTO user_roles (user_id, role_id)
		 SELECT $1, id FROM roles WHERE name = 'user'
		 ON CONFLICT DO NOTHING`,
		userID,
	)
	return err
}

// ---------- Refresh Tokens ----------

func (r *Repository) CreateRefreshToken(ctx context.Context, userID, tokenHash, familyID string, expiresAt time.Time) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO refresh_tokens (user_id, token_hash, family_id, expires_at)
		 VALUES ($1, $2, $3, $4)`,
		userID, tokenHash, familyID, expiresAt,
	)
	return err
}

func (r *Repository) GetRefreshTokenByHash(ctx context.Context, tokenHash string) (*RefreshToken, error) {
	rt := &RefreshToken{}
	err := r.db.QueryRow(ctx,
		`SELECT id, user_id, token_hash, family_id, expires_at, revoked_at, created_at
		 FROM refresh_tokens WHERE token_hash = $1`,
		tokenHash,
	).Scan(&rt.ID, &rt.UserID, &rt.TokenHash, &rt.FamilyID, &rt.ExpiresAt, &rt.RevokedAt, &rt.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return rt, nil
}

func (r *Repository) RevokeRefreshToken(ctx context.Context, id string) error {
	now := time.Now()
	_, err := r.db.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = $1 WHERE id = $2`,
		now, id,
	)
	return err
}

func (r *Repository) RevokeRefreshTokenFamily(ctx context.Context, familyID string) error {
	now := time.Now()
	_, err := r.db.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = $1 WHERE family_id = $2 AND revoked_at IS NULL`,
		now, familyID,
	)
	return err
}

func (r *Repository) RevokeAllUserRefreshTokens(ctx context.Context, userID string) error {
	now := time.Now()
	_, err := r.db.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = $1 WHERE user_id = $2 AND revoked_at IS NULL`,
		now, userID,
	)
	return err
}

// ---------- Password Reset Tokens ----------

func (r *Repository) CreatePasswordResetToken(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO password_reset_tokens (user_id, token_hash, expires_at)
		 VALUES ($1, $2, $3)`,
		userID, tokenHash, expiresAt,
	)
	return err
}

func (r *Repository) GetPasswordResetTokenByHash(ctx context.Context, tokenHash string) (*PasswordResetToken, error) {
	prt := &PasswordResetToken{}
	err := r.db.QueryRow(ctx,
		`SELECT id, user_id, token_hash, expires_at, used_at, created_at
		 FROM password_reset_tokens WHERE token_hash = $1 AND used_at IS NULL`,
		tokenHash,
	).Scan(&prt.ID, &prt.UserID, &prt.TokenHash, &prt.ExpiresAt, &prt.UsedAt, &prt.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return prt, nil
}

func (r *Repository) MarkPasswordResetTokenUsed(ctx context.Context, id string) error {
	now := time.Now()
	_, err := r.db.Exec(ctx,
		`UPDATE password_reset_tokens SET used_at = $1 WHERE id = $2`,
		now, id,
	)
	return err
}

// ---------- OAuth Accounts ----------

func (r *Repository) GetOAuthAccount(ctx context.Context, provider, providerAccountID string) (*OAuthAccount, error) {
	oa := &OAuthAccount{}
	err := r.db.QueryRow(ctx,
		`SELECT id, user_id, provider, provider_account_id, email,
		        access_token_encrypted, refresh_token_encrypted, token_expires_at,
		        created_at, updated_at
		 FROM oauth_accounts
		 WHERE provider = $1 AND provider_account_id = $2`,
		provider, providerAccountID,
	).Scan(&oa.ID, &oa.UserID, &oa.Provider, &oa.ProviderAccountID, &oa.Email,
		&oa.AccessTokenEnc, &oa.RefreshTokenEnc, &oa.TokenExpiresAt,
		&oa.CreatedAt, &oa.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return oa, nil
}

func (r *Repository) CreateOAuthAccount(ctx context.Context, userID, provider, providerAccountID string, email *string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO oauth_accounts (user_id, provider, provider_account_id, email)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (provider, provider_account_id) DO NOTHING`,
		userID, provider, providerAccountID, email,
	)
	return err
}

func (r *Repository) LinkOAuthToUser(ctx context.Context, userID, provider, providerAccountID string, email *string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE oauth_accounts SET user_id = $1, email = COALESCE($4, email), updated_at = now()
		 WHERE provider = $2 AND provider_account_id = $3`,
		userID, provider, providerAccountID, email,
	)
	return err
}

// UUID helper
func newUUID() string {
	return uuid.New().String()
}