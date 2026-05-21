package auth

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	CreateOrganization(ctx context.Context, org *Organization) error
	GetOrganization(ctx context.Context, id string) (*Organization, error)
	SetOnboardingCompleted(ctx context.Context, id string) error
	CreateUser(ctx context.Context, user *User) error
	GetUserByEmail(ctx context.Context, orgID, email string) (*User, error)
	FindUserByEmail(ctx context.Context, email string) (*User, error)
	GetUserByID(ctx context.Context, id string) (*User, error)
	GetUserByProvider(ctx context.Context, provider, providerID string) (*User, error)
	UpdateLastLogin(ctx context.Context, userID string) error
	UpdateMFA(ctx context.Context, userID string, enabled bool, secret string) error
	UpdateUserPassword(ctx context.Context, userID, passwordHash string) error
	StoreRefreshToken(ctx context.Context, userID, token string, expiresAt time.Time) error
	GetRefreshToken(ctx context.Context, tokenHash string) (*RefreshTokenRecord, error)
	DeleteRefreshToken(ctx context.Context, tokenHash string) error
	DeleteUserRefreshTokens(ctx context.Context, userID string) error
	CreateAPIKey(ctx context.Context, key *APIKey) error
	GetAPIKeyByHash(ctx context.Context, hash string) (*APIKey, error)
	RevokeAPIKey(ctx context.Context, id string) error
	StorePasswordResetToken(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error
	GetPasswordResetToken(ctx context.Context, tokenHash string) (*PasswordResetToken, error)
	ConsumePasswordResetToken(ctx context.Context, tokenHash string) error
}

type postgresRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &postgresRepository{pool: pool}
}

func (r *postgresRepository) CreateOrganization(ctx context.Context, org *Organization) error {
	query := `
		INSERT INTO organizations (name, slug, plan, billing_email, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`
	now := time.Now()
	return r.pool.QueryRow(ctx, query, org.Name, org.Slug, org.Plan, org.BillingEmail, now, now).Scan(&org.ID)
}

func (r *postgresRepository) GetOrganization(ctx context.Context, id string) (*Organization, error) {
	query := `
		SELECT id, name, slug, plan, billing_email, COALESCE(onboarding_completed, false), created_at, updated_at, deleted_at
		FROM organizations WHERE id = $1 AND deleted_at IS NULL
	`
	org := &Organization{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&org.ID, &org.Name, &org.Slug, &org.Plan, &org.BillingEmail,
		&org.OnboardingCompleted,
		&org.CreatedAt, &org.UpdatedAt, &org.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	return org, nil
}

func (r *postgresRepository) SetOnboardingCompleted(ctx context.Context, id string) error {
	query := `UPDATE organizations SET onboarding_completed = true, updated_at = $1 WHERE id = $2`
	_, err := r.pool.Exec(ctx, query, time.Now(), id)
	return err
}

func (r *postgresRepository) CreateUser(ctx context.Context, user *User) error {
	query := `
		INSERT INTO users (organization_id, email, password_hash, display_name, avatar_url, role, auth_provider, auth_provider_id, mfa_enabled, mfa_secret, last_login_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id
	`
	now := time.Now()
	return r.pool.QueryRow(ctx, query,
		user.OrganizationID, user.Email, user.PasswordHash, user.DisplayName, user.AvatarURL,
		user.Role, user.AuthProvider, user.AuthProviderID, user.MFAEnabled, user.MFASecret,
		user.LastLoginAt, now, now,
	).Scan(&user.ID)
}

func (r *postgresRepository) GetUserByEmail(ctx context.Context, orgID, email string) (*User, error) {
	query := `
		SELECT id, organization_id, email, password_hash, display_name, avatar_url, role, auth_provider, auth_provider_id,
		       mfa_enabled, mfa_secret, last_login_at, created_at, updated_at, deleted_at
		FROM users WHERE organization_id = $1 AND email = $2 AND deleted_at IS NULL
	`
	return r.scanUser(ctx, query, orgID, email)
}

func (r *postgresRepository) FindUserByEmail(ctx context.Context, email string) (*User, error) {
	query := `
		SELECT id, organization_id, email, password_hash, display_name, avatar_url, role, auth_provider, auth_provider_id,
		       mfa_enabled, mfa_secret, last_login_at, created_at, updated_at, deleted_at
		FROM users WHERE email = $1 AND deleted_at IS NULL
		LIMIT 1
	`
	return r.scanUser(ctx, query, email)
}

func (r *postgresRepository) GetUserByID(ctx context.Context, id string) (*User, error) {
	query := `
		SELECT id, organization_id, email, password_hash, display_name, avatar_url, role, auth_provider, auth_provider_id,
		       mfa_enabled, mfa_secret, last_login_at, created_at, updated_at, deleted_at
		FROM users WHERE id = $1 AND deleted_at IS NULL
	`
	return r.scanUser(ctx, query, id)
}

func (r *postgresRepository) GetUserByProvider(ctx context.Context, provider, providerID string) (*User, error) {
	query := `
		SELECT id, organization_id, email, password_hash, display_name, avatar_url, role, auth_provider, auth_provider_id,
		       mfa_enabled, mfa_secret, last_login_at, created_at, updated_at, deleted_at
		FROM users WHERE auth_provider = $1 AND auth_provider_id = $2 AND deleted_at IS NULL
	`
	return r.scanUser(ctx, query, provider, providerID)
}

func (r *postgresRepository) scanUser(ctx context.Context, query string, args ...any) (*User, error) {
	user := &User{}
	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&user.ID, &user.OrganizationID, &user.Email, &user.PasswordHash, &user.DisplayName, &user.AvatarURL,
		&user.Role, &user.AuthProvider, &user.AuthProviderID, &user.MFAEnabled, &user.MFASecret,
		&user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt, &user.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (r *postgresRepository) UpdateLastLogin(ctx context.Context, userID string) error {
	query := `UPDATE users SET last_login_at = $1, updated_at = $1 WHERE id = $2`
	_, err := r.pool.Exec(ctx, query, time.Now(), userID)
	return err
}

func (r *postgresRepository) UpdateMFA(ctx context.Context, userID string, enabled bool, secret string) error {
	query := `UPDATE users SET mfa_enabled = $1, mfa_secret = $2, updated_at = $3 WHERE id = $4`
	_, err := r.pool.Exec(ctx, query, enabled, secret, time.Now(), userID)
	return err
}

func (r *postgresRepository) UpdateUserPassword(ctx context.Context, userID, passwordHash string) error {
	query := `UPDATE users SET password_hash = $1, updated_at = $2 WHERE id = $3`
	_, err := r.pool.Exec(ctx, query, passwordHash, time.Now(), userID)
	return err
}

func (r *postgresRepository) StoreRefreshToken(ctx context.Context, userID, token string, expiresAt time.Time) error {
	tokenHash := HashToken(token)
	query := `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4)
	`
	_, err := r.pool.Exec(ctx, query, userID, tokenHash, expiresAt, time.Now())
	return err
}

func (r *postgresRepository) GetRefreshToken(ctx context.Context, tokenHash string) (*RefreshTokenRecord, error) {
	query := `
		SELECT id, user_id, token_hash, expires_at, created_at
		FROM refresh_tokens WHERE token_hash = $1 AND expires_at > $2
	`
	record := &RefreshTokenRecord{}
	err := r.pool.QueryRow(ctx, query, tokenHash, time.Now()).Scan(
		&record.ID, &record.UserID, &record.TokenHash, &record.ExpiresAt, &record.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (r *postgresRepository) DeleteRefreshToken(ctx context.Context, tokenHash string) error {
	query := `DELETE FROM refresh_tokens WHERE token_hash = $1`
	_, err := r.pool.Exec(ctx, query, tokenHash)
	return err
}

func (r *postgresRepository) DeleteUserRefreshTokens(ctx context.Context, userID string) error {
	query := `DELETE FROM refresh_tokens WHERE user_id = $1`
	_, err := r.pool.Exec(ctx, query, userID)
	return err
}

func (r *postgresRepository) CreateAPIKey(ctx context.Context, key *APIKey) error {
	query := `
		INSERT INTO api_keys (organization_id, user_id, name, key_prefix, key_hash, scopes, workspace_id, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`
	now := time.Now()
	return r.pool.QueryRow(ctx, query,
		key.OrganizationID, key.UserID, key.Name, key.KeyPrefix, key.KeyHash,
		key.Scopes, key.WorkspaceID, key.ExpiresAt, now,
	).Scan(&key.ID)
}

func (r *postgresRepository) GetAPIKeyByHash(ctx context.Context, hash string) (*APIKey, error) {
	query := `
		SELECT id, organization_id, user_id, name, key_prefix, key_hash, scopes, workspace_id, expires_at, last_used_at, created_at, revoked_at
		FROM api_keys WHERE key_hash = $1 AND revoked_at IS NULL
	`
	key := &APIKey{}
	err := r.pool.QueryRow(ctx, query, hash).Scan(
		&key.ID, &key.OrganizationID, &key.UserID, &key.Name, &key.KeyPrefix, &key.KeyHash,
		&key.Scopes, &key.WorkspaceID, &key.ExpiresAt, &key.LastUsedAt, &key.CreatedAt, &key.RevokedAt,
	)
	if err != nil {
		return nil, err
	}
	return key, nil
}

func (r *postgresRepository) RevokeAPIKey(ctx context.Context, id string) error {
	query := `UPDATE api_keys SET revoked_at = $1 WHERE id = $2`
	_, err := r.pool.Exec(ctx, query, time.Now(), id)
	return err
}

func (r *postgresRepository) StorePasswordResetToken(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	query := `
		INSERT INTO password_reset_tokens (user_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4)
	`
	_, err := r.pool.Exec(ctx, query, userID, tokenHash, expiresAt, time.Now())
	return err
}

func (r *postgresRepository) GetPasswordResetToken(ctx context.Context, tokenHash string) (*PasswordResetToken, error) {
	query := `
		SELECT id, user_id, token_hash, expires_at, used_at, created_at
		FROM password_reset_tokens WHERE token_hash = $1 AND used_at IS NULL AND expires_at > $2
	`
	record := &PasswordResetToken{}
	err := r.pool.QueryRow(ctx, query, tokenHash, time.Now()).Scan(
		&record.ID, &record.UserID, &record.TokenHash, &record.ExpiresAt, &record.UsedAt, &record.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (r *postgresRepository) ConsumePasswordResetToken(ctx context.Context, tokenHash string) error {
	query := `UPDATE password_reset_tokens SET used_at = $1 WHERE token_hash = $2`
	_, err := r.pool.Exec(ctx, query, time.Now(), tokenHash)
	return err
}


