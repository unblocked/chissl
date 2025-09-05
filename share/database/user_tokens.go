package database

import (
	"fmt"
	"time"
)

// CreateUserToken creates a new user token
func (d *SQLDatabase) CreateUserToken(token *UserToken) error {
	query := `INSERT INTO user_tokens (id, username, name, token, created_at, expires_at)
			  VALUES ($1, $2, $3, $4, $5, $6)`

	_, err := d.db.Exec(query, token.ID, token.Username, token.Name, token.Token,
		token.CreatedAt, token.ExpiresAt)
	if err != nil {
		return fmt.Errorf("failed to create user token: %w", err)
	}

	return nil
}

// GetUserToken retrieves a user token by ID
func (d *SQLDatabase) GetUserToken(id string) (*UserToken, error) {
	var token UserToken
	query := `SELECT id, username, name, token, created_at, last_used, expires_at
			  FROM user_tokens WHERE id = $1`

	err := d.db.Get(&token, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get user token: %w", err)
	}

	return &token, nil
}

// ListUserTokens retrieves all tokens for a user
func (d *SQLDatabase) ListUserTokens(username string) ([]*UserToken, error) {
	var tokens []*UserToken
	query := `SELECT id, username, name, token, created_at, last_used, expires_at
			  FROM user_tokens WHERE username = $1 ORDER BY created_at DESC`

	err := d.db.Select(&tokens, query, username)
	if err != nil {
		return nil, fmt.Errorf("failed to list user tokens: %w", err)
	}

	return tokens, nil
}

// DeleteUserToken deletes a user token
func (d *SQLDatabase) DeleteUserToken(id string) error {
	query := `DELETE FROM user_tokens WHERE id = $1`

	result, err := d.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user token: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user token not found")
	}

	return nil
}

// UpdateUserTokenLastUsed updates the last used timestamp for a token
func (d *SQLDatabase) UpdateUserTokenLastUsed(id string, lastUsed time.Time) error {
	query := `UPDATE user_tokens SET last_used = $1 WHERE id = $2`

	_, err := d.db.Exec(query, lastUsed, id)
	if err != nil {
		return fmt.Errorf("failed to update user token last used: %w", err)
	}

	return nil
}

// ValidateUserToken validates a token and returns the associated user token
func (d *SQLDatabase) ValidateUserToken(token string) (*UserToken, error) {
	var userToken UserToken
	query := `SELECT id, username, name, token, created_at, last_used, expires_at
			  FROM user_tokens WHERE token = $1`

	err := d.db.Get(&userToken, query, token)
	if err != nil {
		return nil, fmt.Errorf("invalid token")
	}

	// Check if token is expired
	if userToken.ExpiresAt != nil && time.Now().After(*userToken.ExpiresAt) {
		return nil, fmt.Errorf("token expired")
	}

	// Update last used timestamp
	_ = d.UpdateUserTokenLastUsed(userToken.ID, time.Now())

	return &userToken, nil
}
