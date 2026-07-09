package repo

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"f1/internal/auth/model"
	"f1/internal/auth/service"

	"github.com/lib/pq"
)

type Postgres struct {
	db *sql.DB
}

func NewPostgres(db *sql.DB) *Postgres {
	return &Postgres{db: db}
}

var _ service.AuthRepo = (*Postgres)(nil)

func (p *Postgres) CreateUser(ctx context.Context, email, username, passwordHash string) (int64, error) {
	var id int64
	err := p.db.QueryRowContext(ctx,
		`INSERT INTO users (email, username, password_hash) VALUES ($1, $2, $3) RETURNING id`,
		email, username, passwordHash,
	).Scan(&id)

	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" { // unique_violation
		return 0, service.ErrUserExists
	}
	if err != nil {
		return 0, err
	}

	return id, nil
}

func (p *Postgres) GetUserByLogin(ctx context.Context, login string) (model.User, error) {
	var u model.User
	err := p.db.QueryRowContext(ctx,
		`SELECT id, email, username, password_hash, created_at
		 FROM users WHERE email = $1 OR username = $1`,
		login,
	).Scan(&u.ID, &u.Email, &u.Username, &u.PasswordHash, &u.CreatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return model.User{}, service.ErrNotFound
	}
	if err != nil {
		return model.User{}, err
	}

	return u, nil
}

func (p *Postgres) CreateSession(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time) error {
	_, err := p.db.ExecContext(ctx,
		`INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		userID, tokenHash, expiresAt,
	)
	return err
}

func (p *Postgres) GetSessionByTokenHash(ctx context.Context, tokenHash string) (service.Session, error) {
	var s service.Session
	err := p.db.QueryRowContext(ctx,
		`SELECT id, user_id, token_hash, expires_at, revoked
		 FROM refresh_tokens WHERE token_hash = $1`,
		tokenHash,
	).Scan(&s.ID, &s.UserID, &s.TokenHash, &s.ExpiresAt, &s.Revoked)

	if errors.Is(err, sql.ErrNoRows) {
		return service.Session{}, service.ErrNotFound
	}
	if err != nil {
		return service.Session{}, err
	}

	return s, nil
}

func (p *Postgres) RevokeSession(ctx context.Context, sessionID int64) error {
	_, err := p.db.ExecContext(ctx,
		`UPDATE refresh_tokens SET revoked = TRUE WHERE id = $1`, sessionID)
	return err
}

func (p *Postgres) RevokeAllUserSessions(ctx context.Context, userID int64) error {
	_, err := p.db.ExecContext(ctx,
		`UPDATE refresh_tokens SET revoked = TRUE WHERE user_id = $1 AND NOT revoked`, userID)
	return err
}
