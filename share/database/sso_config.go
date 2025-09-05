package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// SSOProvider represents different SSO providers
type SSOProvider string

const (
	SSOProviderSCIM  SSOProvider = "scim"
	SSOProviderAuth0 SSOProvider = "auth0"
	SSOProviderOkta  SSOProvider = "okta"
	SSOProviderAzure SSOProvider = "azure"
)

// SSOConfig represents SSO configuration stored in database
type SSOConfig struct {
	ID         int         `db:"id" json:"id"`
	Provider   SSOProvider `db:"provider" json:"provider"`
	Enabled    bool        `db:"enabled" json:"enabled"`
	ConfigJSON string      `db:"config_json" json:"-"`
	Config     interface{} `json:"config"`
	CreatedAt  time.Time   `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time   `db:"updated_at" json:"updated_at"`
}

// SCIMConfig represents SCIM-specific configuration
type SCIMConfig struct {
	BaseURL      string            `json:"base_url"`
	ClientID     string            `json:"client_id"`
	ClientSecret string            `json:"client_secret"`
	TenantID     string            `json:"tenant_id,omitempty"`
	AuthURL      string            `json:"auth_url"`
	TokenURL     string            `json:"token_url"`
	UserInfoURL  string            `json:"user_info_url"`
	RedirectURL  string            `json:"redirect_url"`
	Scopes       []string          `json:"scopes"`
	Attributes   map[string]string `json:"attributes"` // Maps SCIM attributes to local fields
}

// Auth0Config represents Auth0-specific configuration (moved from file-based)
type Auth0Config struct {
	Domain       string   `json:"domain"`
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
	RedirectURL  string   `json:"redirect_url"`
	Scopes       []string `json:"scopes"`
}

// UserAuthSource represents how a user was authenticated
type UserAuthSource struct {
	ID           int       `db:"id" json:"id"`
	Username     string    `db:"username" json:"username"`
	AuthSource   string    `db:"auth_source" json:"auth_source"`
	ExternalID   *string   `db:"external_id" json:"external_id,omitempty"`
	ProviderData *string   `db:"provider_data" json:"-"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
}

// CreateSSOConfig creates or updates SSO configuration
func (d *SQLDatabase) CreateSSOConfig(config *SSOConfig) error {
	configJSON, err := json.Marshal(config.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	config.ConfigJSON = string(configJSON)
	config.UpdatedAt = time.Now()

	// Check if config exists for this provider
	var existingID int
	err = d.db.Get(&existingID, "SELECT id FROM sso_config WHERE provider = $1", config.Provider)

	if err == sql.ErrNoRows {
		// Create new config
		config.CreatedAt = time.Now()
		query := `INSERT INTO sso_config (provider, enabled, config_json, created_at, updated_at)
				  VALUES ($1, $2, $3, $4, $5) RETURNING id`
		err = d.db.Get(&config.ID, query, config.Provider, config.Enabled, config.ConfigJSON, config.CreatedAt, config.UpdatedAt)
	} else if err == nil {
		// Update existing config
		config.ID = existingID
		query := `UPDATE sso_config SET enabled = $2, config_json = $3, updated_at = $4 WHERE id = $1`
		_, err = d.db.Exec(query, config.ID, config.Enabled, config.ConfigJSON, config.UpdatedAt)
	}

	if err != nil {
		return fmt.Errorf("failed to save SSO config: %w", err)
	}

	return nil
}

// GetSSOConfig retrieves SSO configuration for a provider
func (d *SQLDatabase) GetSSOConfig(provider SSOProvider) (*SSOConfig, error) {
	config := &SSOConfig{}
	query := `SELECT id, provider, enabled, config_json, created_at, updated_at
			  FROM sso_config WHERE provider = $1`

	err := d.db.Get(config, query, provider)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No config found
		}
		return nil, fmt.Errorf("failed to get SSO config: %w", err)
	}

	// Unmarshal config based on provider
	switch provider {
	case SSOProviderSCIM, SSOProviderOkta:
		var scimConfig SCIMConfig
		if err := json.Unmarshal([]byte(config.ConfigJSON), &scimConfig); err != nil {
			return nil, fmt.Errorf("failed to unmarshal SCIM config: %w", err)
		}
		config.Config = scimConfig
	case SSOProviderAuth0:
		var auth0Config Auth0Config
		if err := json.Unmarshal([]byte(config.ConfigJSON), &auth0Config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal Auth0 config: %w", err)
		}
		config.Config = auth0Config
	}

	return config, nil
}

// ListSSOConfigs returns all SSO configurations
func (d *SQLDatabase) ListSSOConfigs() ([]*SSOConfig, error) {
	var configs []*SSOConfig
	query := `SELECT id, provider, enabled, config_json, created_at, updated_at
			  FROM sso_config ORDER BY provider`

	rows, err := d.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list SSO configs: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		config := &SSOConfig{}
		err := rows.Scan(&config.ID, &config.Provider, &config.Enabled, &config.ConfigJSON, &config.CreatedAt, &config.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan SSO config: %w", err)
		}

		// Unmarshal config based on provider
		switch config.Provider {
		case SSOProviderSCIM, SSOProviderOkta:
			var scimConfig SCIMConfig
			if err := json.Unmarshal([]byte(config.ConfigJSON), &scimConfig); err == nil {
				config.Config = scimConfig
			}
		case SSOProviderAuth0:
			var auth0Config Auth0Config
			if err := json.Unmarshal([]byte(config.ConfigJSON), &auth0Config); err == nil {
				config.Config = auth0Config
			}
		}

		configs = append(configs, config)
	}

	return configs, nil
}

// DeleteSSOConfig deletes SSO configuration for a provider
func (d *SQLDatabase) DeleteSSOConfig(provider SSOProvider) error {
	query := `DELETE FROM sso_config WHERE provider = $1`
	_, err := d.db.Exec(query, provider)
	if err != nil {
		return fmt.Errorf("failed to delete SSO config: %w", err)
	}
	return nil
}

// CreateUserAuthSource creates a user authentication source record
func (d *SQLDatabase) CreateUserAuthSource(source *UserAuthSource) error {
	source.CreatedAt = time.Now()
	source.UpdatedAt = time.Now()

	query := `INSERT INTO user_auth_sources (username, auth_source, external_id, provider_data, created_at, updated_at)
			  VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`

	err := d.db.Get(&source.ID, query, source.Username, source.AuthSource, source.ExternalID, source.ProviderData, source.CreatedAt, source.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create user auth source: %w", err)
	}

	return nil
}

// GetUserAuthSource retrieves user authentication source
func (d *SQLDatabase) GetUserAuthSource(username string) (*UserAuthSource, error) {
	source := &UserAuthSource{}
	query := `SELECT id, username, auth_source, external_id, provider_data, created_at, updated_at
			  FROM user_auth_sources WHERE username = $1`

	err := d.db.Get(source, query, username)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No auth source found (local user)
		}
		return nil, fmt.Errorf("failed to get user auth source: %w", err)
	}

	return source, nil
}

// UpdateUserAuthSource updates user authentication source
func (d *SQLDatabase) UpdateUserAuthSource(source *UserAuthSource) error {
	source.UpdatedAt = time.Now()

	query := `UPDATE user_auth_sources SET auth_source = $2, external_id = $3, provider_data = $4, updated_at = $5
			  WHERE username = $1`

	_, err := d.db.Exec(query, source.Username, source.AuthSource, source.ExternalID, source.ProviderData, source.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to update user auth source: %w", err)
	}

	return nil
}

// ListUserAuthSources returns all user authentication sources
func (d *SQLDatabase) ListUserAuthSources() ([]*UserAuthSource, error) {
	var sources []*UserAuthSource
	query := `SELECT id, username, auth_source, external_id, provider_data, created_at, updated_at
			  FROM user_auth_sources ORDER BY username`

	err := d.db.Select(&sources, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list user auth sources: %w", err)
	}

	return sources, nil
}

// ListUserAuthSourcesByUsername returns authentication sources for a specific user
func (d *SQLDatabase) ListUserAuthSourcesByUsername(username string) ([]*UserAuthSource, error) {
	var sources []*UserAuthSource
	query := `SELECT id, username, auth_source, external_id, provider_data, created_at, updated_at
			  FROM user_auth_sources WHERE username = $1 ORDER BY created_at`

	err := d.db.Select(&sources, query, username)
	if err != nil {
		return nil, fmt.Errorf("failed to list user auth sources for %s: %w", username, err)
	}

	return sources, nil
}

// DeleteUserAuthSource deletes a user authentication source by ID
func (d *SQLDatabase) DeleteUserAuthSource(id int) error {
	query := `DELETE FROM user_auth_sources WHERE id = $1`

	result, err := d.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user auth source: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user auth source not found: %d", id)
	}

	return nil
}
