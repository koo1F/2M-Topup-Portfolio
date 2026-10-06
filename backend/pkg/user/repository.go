package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/lib/pq"
)

// ErrEmailTaken is returned when the email is already registered.
var ErrEmailTaken = errors.New("email already taken")

// Repository is the interface for user-related data access.
type Repository interface {
	// CreateUserWithWallet inserts a user and creates their wallet inside a single DB transaction.
	CreateUserWithWallet(ctx context.Context, email, passwordHash string) (*User, error)
	// GetUserByEmail retrieves a user by their email address.
	GetUserByEmail(ctx context.Context, email string) (*User, error)
}

// pgRepository is the PostgreSQL implementation of Repository.
type pgRepository struct {
	db *sql.DB
}

// NewRepository creates a new pgRepository backed by the given *sql.DB.
func NewRepository(db *sql.DB) Repository {
	return &pgRepository{db: db}
}

// CreateUserWithWallet inserts the user and their wallet atomically.
func (r *pgRepository) CreateUserWithWallet(ctx context.Context, email, passwordHash string) (*User, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback is a no-op after commit

	// Insert user
	var u User
	const insertUser = `
		INSERT INTO users (email, password_hash)
		VALUES ($1, $2)
		RETURNING id, email, is_active, created_at, updated_at`

	err = tx.QueryRowContext(ctx, insertUser, email, passwordHash).
		Scan(&u.ID, &u.Email, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrEmailTaken
		}
		return nil, fmt.Errorf("insert user: %w", err)
	}

	// Insert wallet for the new user (balance defaults to 0.00)
	const insertWallet = `INSERT INTO wallets (user_id) VALUES ($1)`
	if _, err = tx.ExecContext(ctx, insertWallet, u.ID); err != nil {
		return nil, fmt.Errorf("insert wallet: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return &u, nil
}

// GetUserByEmail retrieves the user (including password_hash for login comparison).
func (r *pgRepository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	const query = `
		SELECT id, email, password_hash, is_active, created_at, updated_at
		FROM users
		WHERE email = $1`

	var u User
	err := r.db.QueryRowContext(ctx, query, email).
		Scan(&u.ID, &u.Email, &u.PasswordHash, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil // caller checks nil to detect "not found"
	}
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}

	return &u, nil
}

// isUniqueViolation detects PostgreSQL error code 23505 (unique_violation).
func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23505"
}
