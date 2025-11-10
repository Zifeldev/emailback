package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Zifeldev/emailback/service/Auth/internal/config"
	"github.com/Zifeldev/emailback/service/Auth/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUserNotFound  = errors.New("user not found")
	ErrUserExists    = errors.New("user already exists")
	ErrTokenNotFound = errors.New("refresh token not found")
	ErrTokenRevoked  = errors.New("refresh token revoked")
	ErrTokenExpired  = errors.New("refresh token expired")
)

type UserRepository interface {
	Create(ctx context.Context, email, passwordHash string) (*models.User, error)
	CreateWithRole(ctx context.Context, email, passwordHash, role string) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	GetByID(ctx context.Context, id int64) (*models.User, error)
	UpdateRole(ctx context.Context, id int64, role string) (*models.User, error)
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, limit, offset int) ([]*models.User, error)
}

type TokenRepository interface {
	CreateRefreshToken(ctx context.Context, userID int64, token string, expiresAt time.Time) (*models.RefreshToken, error)
	GetRefreshToken(ctx context.Context, token string) (*models.RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, token string) error
	RevokeAllUserTokens(ctx context.Context, userID int64) error
	CleanupExpiredTokens(ctx context.Context) error
}

type userRepository struct {
	pool *pgxpool.Pool
	cfg  *config.JWTConfig
}

type tokenRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool, cfg *config.JWTConfig) UserRepository {
	return &userRepository{
		pool: pool,
		cfg:  cfg,
	}
}

func NewTokenRepository(pool *pgxpool.Pool) TokenRepository {
	return &tokenRepository{pool: pool}
}

// UserRepository implementation

func (r *userRepository) Create(ctx context.Context, email, passwordHash string) (*models.User, error) {
	user := &models.User{}

	role := models.RoleUser
	if r.cfg.FirstAdminEmail != "" && email == r.cfg.FirstAdminEmail {
		role = models.RoleAdmin
	}

	query := `
		INSERT INTO users (email, password_hash, role, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		RETURNING id, email, password_hash, role, created_at, updated_at
	`

	err := r.pool.QueryRow(ctx, query, email, passwordHash, role).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if err.Error() == "duplicate key value violates unique constraint \"users_email_key\"" {
			return nil, ErrUserExists
		}
		return nil, err
	}

	return user, nil
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	user := &models.User{}
	query := `SELECT id, email, password_hash, role, created_at, updated_at FROM users WHERE email = $1`

	err := r.pool.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return user, nil
}

func (r *userRepository) GetByID(ctx context.Context, id int64) (*models.User, error) {
	user := &models.User{}
	query := `SELECT id, email, password_hash, role, created_at, updated_at FROM users WHERE id = $1`

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return user, nil
}

func (r *userRepository) CreateWithRole(ctx context.Context, email, passwordHash, role string) (*models.User, error) {
	user := &models.User{}

	// Validate role
	if err := models.ValidateRole(role); err != nil {
		return nil, err
	}

	query := `
		INSERT INTO users (email, password_hash, role, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		RETURNING id, email, password_hash, role, created_at, updated_at
	`

	err := r.pool.QueryRow(ctx, query, email, passwordHash, role).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if err.Error() == "duplicate key value violates unique constraint \"users_email_key\"" {
			return nil, ErrUserExists
		}
		return nil, err
	}

	return user, nil
}

func (r *userRepository) UpdateRole(ctx context.Context, id int64, role string) (*models.User, error) {
	// Validate role
	if err := models.ValidateRole(role); err != nil {
		return nil, err
	}

	user := &models.User{}
	query := `
		UPDATE users 
		SET role = $2, updated_at = NOW() 
		WHERE id = $1
		RETURNING id, email, password_hash, role, created_at, updated_at
	`

	err := r.pool.QueryRow(ctx, query, id, role).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return user, nil
}

func (r *userRepository) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM users WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrUserNotFound
	}

	return nil
}

func (r *userRepository) List(ctx context.Context, limit, offset int) ([]*models.User, error) {
	query := `
		SELECT id, email, password_hash, role, created_at, updated_at 
		FROM users 
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]*models.User, 0)
	for rows.Next() {
		user := &models.User{}
		err := rows.Scan(
			&user.ID,
			&user.Email,
			&user.PasswordHash,
			&user.Role,
			&user.CreatedAt,
			&user.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

// TokenRepository implementation

func (r *tokenRepository) CreateRefreshToken(ctx context.Context, userID int64, token string, expiresAt time.Time) (*models.RefreshToken, error) {
	rt := &models.RefreshToken{}
	query := `
		INSERT INTO refresh_tokens (user_id, token, expires_at, created_at, revoked)
		VALUES ($1, $2, $3, NOW(), FALSE)
		RETURNING id, user_id, token, expires_at, created_at, revoked
	`

	err := r.pool.QueryRow(ctx, query, userID, token, expiresAt).Scan(
		&rt.ID,
		&rt.UserID,
		&rt.Token,
		&rt.ExpiresAt,
		&rt.CreatedAt,
		&rt.Revoked,
	)

	if err != nil {
		return nil, err
	}

	return rt, nil
}

func (r *tokenRepository) GetRefreshToken(ctx context.Context, token string) (*models.RefreshToken, error) {
	rt := &models.RefreshToken{}
	query := `SELECT id, user_id, token, expires_at, created_at, revoked FROM refresh_tokens WHERE token = $1`

	err := r.pool.QueryRow(ctx, query, token).Scan(
		&rt.ID,
		&rt.UserID,
		&rt.Token,
		&rt.ExpiresAt,
		&rt.CreatedAt,
		&rt.Revoked,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTokenNotFound
		}
		return nil, err
	}

	if rt.Revoked {
		return nil, ErrTokenRevoked
	}

	if time.Now().After(rt.ExpiresAt) {
		return nil, ErrTokenExpired
	}

	return rt, nil
}

func (r *tokenRepository) RevokeRefreshToken(ctx context.Context, token string) error {
	query := `UPDATE refresh_tokens SET revoked = TRUE WHERE token = $1`
	_, err := r.pool.Exec(ctx, query, token)
	return err
}

func (r *tokenRepository) RevokeAllUserTokens(ctx context.Context, userID int64) error {
	query := `UPDATE refresh_tokens SET revoked = TRUE WHERE user_id = $1 AND revoked = FALSE`
	_, err := r.pool.Exec(ctx, query, userID)
	return err
}

func (r *tokenRepository) CleanupExpiredTokens(ctx context.Context) error {
	query := `DELETE FROM refresh_tokens WHERE expires_at < NOW() OR revoked = TRUE`
	_, err := r.pool.Exec(ctx, query)
	return err
}
