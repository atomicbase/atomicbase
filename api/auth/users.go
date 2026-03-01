package auth

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type User struct {
	ID              string     `json:"id"`
	Email           string     `json:"email"`
	EmailVerifiedAt *time.Time `json:"email_verified_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

func FindOrCreateUser(email string, db *sql.DB, ctx context.Context) (*User, bool, error) {
	email = NormalizeEmail(email)
	now := time.Now().UTC()

	// Try to find existing user
	var user User
	var emailVerifiedAt sql.NullString
	var createdAt string
	err := db.QueryRowContext(ctx,
		`SELECT id, email, email_verified_at, created_at
		 FROM atombase_users WHERE email = ?`,
		email,
	).Scan(&user.ID, &user.Email, &emailVerifiedAt, &createdAt)

	if err == nil {
		user.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		if emailVerifiedAt.Valid {
			if t, err := time.Parse(time.RFC3339, emailVerifiedAt.String); err == nil {
				user.EmailVerifiedAt = &t
			}
		}

		// Existing user - mark email as verified and update last sign in
		nowStr := now.Format(time.RFC3339)
		if user.EmailVerifiedAt == nil {
			db.ExecContext(ctx,
				`UPDATE atombase_users SET email_verified_at = ?, last_sign_in_at = ? WHERE id = ?`,
				nowStr, nowStr, user.ID,
			)
			user.EmailVerifiedAt = &now
		} else {
			db.ExecContext(ctx,
				`UPDATE atombase_users SET last_sign_in_at = ? WHERE id = ?`,
				nowStr, user.ID,
			)
		}
		return &user, false, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return nil, false, err
	}

	// Create new user
	nowStr := now.Format(time.RFC3339)
	user = User{
		ID:              ID128(),
		Email:           email,
		EmailVerifiedAt: &now,
		CreatedAt:       now,
	}

	_, err = db.ExecContext(ctx,
		`INSERT INTO atombase_users (id, email, email_verified_at, last_sign_in_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		user.ID, user.Email, nowStr, nowStr, nowStr, nowStr,
	)
	if err != nil {
		return nil, false, err
	}

	return &user, true, nil
}

func GetUserByID(userID string, db *sql.DB, ctx context.Context) (*User, error) {
	var user User
	var emailVerifiedAt sql.NullString
	var createdAt string

	err := db.QueryRowContext(ctx,
		`SELECT id, email, email_verified_at, created_at
		 FROM atombase_users WHERE id = ?`,
		userID,
	).Scan(&user.ID, &user.Email, &emailVerifiedAt, &createdAt)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrInvalidSession
	}
	if err != nil {
		return nil, err
	}

	user.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if emailVerifiedAt.Valid {
		if t, err := time.Parse(time.RFC3339, emailVerifiedAt.String); err == nil {
			user.EmailVerifiedAt = &t
		}
	}

	return &user, nil
}
