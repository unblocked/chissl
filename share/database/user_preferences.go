package database

import (
	"database/sql"
	"fmt"
	"time"
)

// UserPreference represents a user preference in the database
type UserPreference struct {
	ID              int       `db:"id" json:"id"`
	Username        string    `db:"username" json:"username"`
	PreferenceKey   string    `db:"preference_key" json:"preference_key"`
	PreferenceValue string    `db:"preference_value" json:"preference_value"`
	CreatedAt       time.Time `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time `db:"updated_at" json:"updated_at"`
}

// GetUserPreference retrieves a user preference by username and key
func (d *SQLDatabase) GetUserPreference(username, key string) (*UserPreference, error) {
	pref := &UserPreference{}
	query := `SELECT id, username, preference_key, preference_value, created_at, updated_at
			  FROM user_preferences WHERE username = $1 AND preference_key = $2`

	err := d.db.Get(pref, query, username, key)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No preference found
		}
		return nil, fmt.Errorf("failed to get user preference: %w", err)
	}

	return pref, nil
}

// SetUserPreference creates or updates a user preference
func (d *SQLDatabase) SetUserPreference(username, key, value string) error {
	now := time.Now()

	// Try to update existing preference first
	updateQuery := `UPDATE user_preferences 
					SET preference_value = $1, updated_at = $2 
					WHERE username = $3 AND preference_key = $4`

	result, err := d.db.Exec(updateQuery, value, now, username, key)
	if err != nil {
		return fmt.Errorf("failed to update user preference: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	// If no rows were updated, insert new preference
	if rowsAffected == 0 {
		insertQuery := `INSERT INTO user_preferences (username, preference_key, preference_value, created_at, updated_at)
						VALUES ($1, $2, $3, $4, $5)`

		_, err = d.db.Exec(insertQuery, username, key, value, now, now)
		if err != nil {
			return fmt.Errorf("failed to create user preference: %w", err)
		}
	}

	return nil
}

// DeleteUserPreference deletes a user preference
func (d *SQLDatabase) DeleteUserPreference(username, key string) error {
	query := `DELETE FROM user_preferences WHERE username = $1 AND preference_key = $2`

	_, err := d.db.Exec(query, username, key)
	if err != nil {
		return fmt.Errorf("failed to delete user preference: %w", err)
	}

	return nil
}

// ListUserPreferences retrieves all preferences for a user
func (d *SQLDatabase) ListUserPreferences(username string) ([]*UserPreference, error) {
	var prefs []*UserPreference
	query := `SELECT id, username, preference_key, preference_value, created_at, updated_at
			  FROM user_preferences WHERE username = $1 ORDER BY preference_key`

	err := d.db.Select(&prefs, query, username)
	if err != nil {
		return nil, fmt.Errorf("failed to list user preferences: %w", err)
	}

	return prefs, nil
}
