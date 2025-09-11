package database

import (
	"database/sql"
	"fmt"
	"time"
)

// SecurityWebhook represents a webhook for security events
type SecurityWebhook struct {
	ID          int       `db:"id" json:"id"`
	URL         string    `db:"url" json:"url"`
	Type        string    `db:"type" json:"type"` // "slack" or "json"
	Enabled     bool      `db:"enabled" json:"enabled"`
	Description string    `db:"description" json:"description"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

// ListSecurityWebhooks returns all webhooks (optionally only enabled)
func (d *SQLDatabase) ListSecurityWebhooks(onlyEnabled bool) ([]SecurityWebhook, error) {
	var rows []SecurityWebhook
	var query string
	if onlyEnabled {
		if d.config.Type == "postgres" {
			query = `SELECT id, url, type, enabled, description, created_at, updated_at FROM security_webhooks WHERE enabled = TRUE ORDER BY id ASC`
		} else {
			query = `SELECT id, url, type, enabled, description, created_at, updated_at FROM security_webhooks WHERE enabled = 1 ORDER BY id ASC`
		}
	} else {
		query = `SELECT id, url, type, enabled, description, created_at, updated_at FROM security_webhooks ORDER BY id ASC`
	}
	err := d.db.Select(&rows, query)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	return rows, nil
}

// GetSecurityWebhook returns a single webhook by id
func (d *SQLDatabase) GetSecurityWebhook(id int) (*SecurityWebhook, error) {
	var row SecurityWebhook
	var err error
	if d.config.Type == "postgres" {
		err = d.db.Get(&row, `SELECT id, url, type, enabled, description, created_at, updated_at FROM security_webhooks WHERE id = $1`, id)
	} else {
		err = d.db.Get(&row, `SELECT id, url, type, enabled, description, created_at, updated_at FROM security_webhooks WHERE id = ?`, id)
	}
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("webhook not found")
		}
		return nil, err
	}
	return &row, nil
}

// CreateSecurityWebhook inserts a new webhook
func (d *SQLDatabase) CreateSecurityWebhook(w *SecurityWebhook) error {
	now := time.Now()
	w.CreatedAt = now
	w.UpdatedAt = now
	// SQLite uses last_insert_rowid; Postgres RETURNING id
	switch d.config.Type {
	case "postgres":
		return d.db.Get(&w.ID, `INSERT INTO security_webhooks (url, type, enabled, description, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`, w.URL, w.Type, w.Enabled, w.Description, w.CreatedAt, w.UpdatedAt)
	default:
		res, err := d.db.Exec(`INSERT INTO security_webhooks (url, type, enabled, description, created_at, updated_at) VALUES (?,?,?,?,?,?)`, w.URL, w.Type, w.Enabled, w.Description, w.CreatedAt, w.UpdatedAt)
		if err != nil {
			return err
		}
		if id, err := res.LastInsertId(); err == nil {
			w.ID = int(id)
		}
		return nil
	}
}

// UpdateSecurityWebhook updates an existing webhook
func (d *SQLDatabase) UpdateSecurityWebhook(w *SecurityWebhook) error {
	w.UpdatedAt = time.Now()
	var err error
	if d.config.Type == "postgres" {
		_, err = d.db.Exec(`UPDATE security_webhooks SET url = $1, type = $2, enabled = $3, description = $4, updated_at = $5 WHERE id = $6`, w.URL, w.Type, w.Enabled, w.Description, w.UpdatedAt, w.ID)
	} else {
		_, err = d.db.Exec(`UPDATE security_webhooks SET url = ?, type = ?, enabled = ?, description = ?, updated_at = ? WHERE id = ?`, w.URL, w.Type, w.Enabled, w.Description, w.UpdatedAt, w.ID)
	}
	if err != nil {
		return fmt.Errorf("failed to update webhook: %w", err)
	}
	return nil
}

// DeleteSecurityWebhook deletes a webhook by id
func (d *SQLDatabase) DeleteSecurityWebhook(id int) error {
	var err error
	if d.config.Type == "postgres" {
		_, err = d.db.Exec(`DELETE FROM security_webhooks WHERE id = $1`, id)
	} else {
		_, err = d.db.Exec(`DELETE FROM security_webhooks WHERE id = ?`, id)
	}
	if err != nil {
		return fmt.Errorf("failed to delete webhook: %w", err)
	}
	return nil
}
