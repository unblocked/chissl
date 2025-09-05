package database

import (
	"database/sql"
	"fmt"
	"time"
)

// GetUser retrieves a user by username
func (d *SQLDatabase) GetUser(username string) (*User, error) {
	user := &User{}
	query := `SELECT id, username, password, email, display_name, is_admin, addresses, created_at, updated_at
			  FROM users WHERE username = $1`

	err := d.db.Get(user, query, username)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found: %s", username)
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// CreateUser creates a new user
func (d *SQLDatabase) CreateUser(user *User) error {
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()

	query := `INSERT INTO users (username, password, email, display_name, is_admin, addresses, created_at, updated_at)
			  VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`

	err := d.db.QueryRow(query, user.Username, user.Password, user.Email, user.DisplayName, user.IsAdmin,
		user.Addresses, user.CreatedAt, user.UpdatedAt).Scan(&user.ID)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// UpdateUser updates an existing user
func (d *SQLDatabase) UpdateUser(user *User) error {
	user.UpdatedAt = time.Now()

	query := `UPDATE users SET password = $1, email = $2, display_name = $3, is_admin = $4, addresses = $5, updated_at = $6
			  WHERE username = $7`

	result, err := d.db.Exec(query, user.Password, user.Email, user.DisplayName, user.IsAdmin, user.Addresses,
		user.UpdatedAt, user.Username)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found: %s", user.Username)
	}

	return nil
}

// DeleteUser deletes a user by username
func (d *SQLDatabase) DeleteUser(username string) error {
	query := `DELETE FROM users WHERE username = $1`

	result, err := d.db.Exec(query, username)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found: %s", username)
	}

	return nil
}

// ListUsers retrieves all users
func (d *SQLDatabase) ListUsers() ([]*User, error) {
	var users []*User
	query := `SELECT id, username, password, email, display_name, is_admin, addresses, created_at, updated_at
			  FROM users ORDER BY username`

	err := d.db.Select(&users, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	return users, nil
}

// CreateSession creates a new user session
func (d *SQLDatabase) CreateSession(session *Session) error {
	session.CreatedAt = time.Now()

	query := `INSERT INTO sessions (id, username, created_at, expires_at, ip_address) 
			  VALUES ($1, $2, $3, $4, $5)`

	_, err := d.db.Exec(query, session.ID, session.Username, session.CreatedAt,
		session.ExpiresAt, session.IPAddress)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	return nil
}

// GetSession retrieves a session by ID
func (d *SQLDatabase) GetSession(sessionID string) (*Session, error) {
	session := &Session{}
	query := `SELECT id, username, created_at, expires_at, ip_address 
			  FROM sessions WHERE id = $1 AND expires_at > $2`

	err := d.db.Get(session, query, sessionID, time.Now())
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("session not found or expired: %s", sessionID)
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return session, nil
}

// DeleteSession deletes a session by ID
func (d *SQLDatabase) DeleteSession(sessionID string) error {
	query := `DELETE FROM sessions WHERE id = $1`

	_, err := d.db.Exec(query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	return nil
}

// CleanupExpiredSessions removes expired sessions
func (d *SQLDatabase) CleanupExpiredSessions() error {
	query := `DELETE FROM sessions WHERE expires_at <= $1`

	_, err := d.db.Exec(query, time.Now())
	if err != nil {
		return fmt.Errorf("failed to cleanup expired sessions: %w", err)
	}

	return nil
}
