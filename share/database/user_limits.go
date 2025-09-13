package database

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// UserLimits represents user-specific limits
type UserLimits struct {
	Username     string    `db:"username" json:"username"`
	MaxTunnels   *int      `db:"max_tunnels" json:"max_tunnels"`
	MaxListeners *int      `db:"max_listeners" json:"max_listeners"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
}

// CreateUserLimits creates or updates user limits
func (d *SQLDatabase) CreateUserLimits(limits *UserLimits) error {
	limits.CreatedAt = time.Now()
	limits.UpdatedAt = time.Now()

	query := `INSERT OR REPLACE INTO user_limits (username, max_tunnels, max_listeners, created_at, updated_at)
			  VALUES ($1, $2, $3, $4, $5)`

	_, err := d.db.Exec(query, limits.Username, limits.MaxTunnels, limits.MaxListeners,
		limits.CreatedAt, limits.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create user limits: %w", err)
	}

	return nil
}

// GetUserLimits retrieves limits for a specific user
func (d *SQLDatabase) GetUserLimits(username string) (*UserLimits, error) {
	limits := &UserLimits{}
	query := `SELECT username, max_tunnels, max_listeners, created_at, updated_at
			  FROM user_limits WHERE username = $1`

	err := d.db.Get(limits, query, username)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No custom limits set
		}
		return nil, fmt.Errorf("failed to get user limits: %w", err)
	}

	return limits, nil
}

// UpdateUserLimits updates existing user limits
func (d *SQLDatabase) UpdateUserLimits(limits *UserLimits) error {
	limits.UpdatedAt = time.Now()

	query := `UPDATE user_limits SET max_tunnels = $2, max_listeners = $3, updated_at = $4
			  WHERE username = $1`

	result, err := d.db.Exec(query, limits.Username, limits.MaxTunnels, limits.MaxListeners, limits.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to update user limits: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user limits not found")
	}

	return nil
}

// DeleteUserLimits removes user limits (reverts to defaults)
func (d *SQLDatabase) DeleteUserLimits(username string) error {
	query := `DELETE FROM user_limits WHERE username = $1`

	_, err := d.db.Exec(query, username)
	if err != nil {
		return fmt.Errorf("failed to delete user limits: %w", err)
	}

	return nil
}

// GetEffectiveUserLimits gets the effective limits for a user (custom or default)
func (d *SQLDatabase) GetEffectiveUserLimits(username string) (maxTunnels, maxListeners int, err error) {
	// First try to get user-specific limits
	userLimits, err := d.GetUserLimits(username)
	if err != nil {
		return 0, 0, err
	}

	// Get default limits from settings
	defaultMaxTunnels, err := d.GetSettingInt("default_max_tunnels", 10)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get default max tunnels: %w", err)
	}

	defaultMaxListeners, err := d.GetSettingInt("default_max_listeners", 5)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get default max listeners: %w", err)
	}

	// Use user-specific limits if set, otherwise use defaults
	if userLimits != nil && userLimits.MaxTunnels != nil {
		maxTunnels = *userLimits.MaxTunnels
	} else {
		maxTunnels = defaultMaxTunnels
	}

	if userLimits != nil && userLimits.MaxListeners != nil {
		maxListeners = *userLimits.MaxListeners
	} else {
		maxListeners = defaultMaxListeners
	}

	return maxTunnels, maxListeners, nil
}

// CheckUserTunnelLimit checks if user can create another tunnel
func (d *SQLDatabase) CheckUserTunnelLimit(username string) (bool, error) {
	maxTunnels, _, err := d.GetEffectiveUserLimits(username)
	if err != nil {
		return false, err
	}

	// Count current tunnels
	var currentCount int
	err = d.db.Get(&currentCount, "SELECT COUNT(*) FROM tunnels WHERE username = $1", username)
	if err != nil {
		return false, fmt.Errorf("failed to count user tunnels: %w", err)
	}

	return currentCount < maxTunnels, nil
}

// CheckUserListenerLimit checks if user can create another listener
func (d *SQLDatabase) CheckUserListenerLimit(username string) (bool, error) {
	_, maxListeners, err := d.GetEffectiveUserLimits(username)
	if err != nil {
		return false, err
	}

	// Count current listeners
	var currentCount int
	err = d.db.Get(&currentCount, "SELECT COUNT(*) FROM listeners WHERE username = $1", username)
	if err != nil {
		return false, fmt.Errorf("failed to count user listeners: %w", err)
	}

	return currentCount < maxListeners, nil
}

// GetSettingInt is a helper to get integer settings with default
func (d *SQLDatabase) GetSettingInt(key string, defaultValue int) (int, error) {
	var value string
	query := `SELECT value FROM settings WHERE key = $1`

	err := d.db.Get(&value, query, key)
	if err != nil {
		if err == sql.ErrNoRows {
			return defaultValue, nil
		}
		return 0, err
	}

	// Convert string to int
	var intValue int
	_, err = fmt.Sscanf(value, "%d", &intValue)
	if err != nil {
		return defaultValue, nil // Return default if conversion fails
	}

	return intValue, nil
}

// GetSettingBool is a helper to get boolean settings with default
func (d *SQLDatabase) GetSettingBool(key string, defaultValue bool) (bool, error) {
	var value string
	query := `SELECT value FROM settings WHERE key = $1`
	if err := d.db.Get(&value, query, key); err != nil {
		if err == sql.ErrNoRows {
			return defaultValue, nil
		}
		return defaultValue, err
	}
	// Normalize and parse common truthy/falsey values
	switch valueLower := strings.ToLower(strings.TrimSpace(value)); valueLower {
	case "1", "true", "yes", "on", "enabled":
		return true, nil
	case "0", "false", "no", "off", "disabled":
		return false, nil
	default:
		return defaultValue, nil
	}
}

// SetSettingString upserts a string setting value
func (d *SQLDatabase) SetSettingString(key string, value string) error {
	query := `INSERT INTO settings (key, value, updated_at) VALUES ($1, $2, $3)
			  ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = $3`
	_, err := d.db.Exec(query, key, value, time.Now())
	if err != nil {
		return fmt.Errorf("failed to set setting %s: %w", key, err)
	}
	return nil
}
