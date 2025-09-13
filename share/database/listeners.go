package database

import (
	"fmt"
	"time"
)

// CreateListener creates a new listener in the database
func (d *SQLDatabase) CreateListener(listener *Listener) error {
	listener.CreatedAt = time.Now()
	listener.UpdatedAt = time.Now()

	query := `INSERT INTO listeners (id, name, username, port, mode, target_url, response, use_tls, status, created_at, updated_at, bytes_sent, bytes_recv, connections)
			  VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`

	_, err := d.db.Exec(query, listener.ID, listener.Name, listener.Username, listener.Port, listener.Mode,
		listener.TargetURL, listener.Response, listener.UseTLS, listener.Status, listener.CreatedAt, listener.UpdatedAt,
		listener.BytesSent, listener.BytesRecv, listener.Connections)

	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}

	return nil
}

// UpdateListener updates an existing listener in the database
func (d *SQLDatabase) UpdateListener(listener *Listener) error {
	listener.UpdatedAt = time.Now()

	query := `UPDATE listeners SET name = COALESCE($1, name), status = COALESCE($2, status), updated_at = $3, bytes_sent = COALESCE($4, bytes_sent),
			  bytes_recv = COALESCE($5, bytes_recv), connections = COALESCE($6, connections),
			  target_url = COALESCE($7, target_url), response = COALESCE($8, response), use_tls = COALESCE($9, use_tls) WHERE id = $10`

	result, err := d.db.Exec(query, listener.Name, listener.Status, listener.UpdatedAt, listener.BytesSent,
		listener.BytesRecv, listener.Connections, listener.TargetURL, listener.Response, listener.UseTLS, listener.ID)

	if err != nil {
		return fmt.Errorf("failed to update listener: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("listener not found: %s", listener.ID)
	}

	return nil
}

// DeleteListener removes a listener from the database
func (d *SQLDatabase) DeleteListener(listenerID string) error {
	query := `DELETE FROM listeners WHERE id = $1`

	result, err := d.db.Exec(query, listenerID)
	if err != nil {
		return fmt.Errorf("failed to delete listener: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("listener not found: %s", listenerID)
	}

	return nil
}

// GetListener retrieves a listener by ID
func (d *SQLDatabase) GetListener(listenerID string) (*Listener, error) {
	var listener Listener
	query := `SELECT id, name, username, port, mode, target_url, response, use_tls, status, created_at, updated_at, bytes_sent, bytes_recv, connections
			  FROM listeners WHERE id = $1`

	err := d.db.Get(&listener, query, listenerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get listener: %w", err)
	}

	return &listener, nil
}

// ListListeners retrieves all listeners
func (d *SQLDatabase) ListListeners() ([]*Listener, error) {
	var listeners []*Listener
	query := `SELECT id, name, username, port, mode, target_url, response, use_tls, status, created_at, updated_at, bytes_sent, bytes_recv, connections
			  FROM listeners ORDER BY created_at DESC`

	err := d.db.Select(&listeners, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list listeners: %w", err)
	}

	return listeners, nil
}

// ListActiveListeners retrieves all active listeners
func (d *SQLDatabase) ListActiveListeners() ([]*Listener, error) {
	var listeners []*Listener
	query := `SELECT id, username, port, mode, target_url, response, use_tls, status, created_at, updated_at, bytes_sent, bytes_recv, connections
			  FROM listeners WHERE status = 'open' ORDER BY created_at DESC`

	err := d.db.Select(&listeners, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list active listeners: %w", err)
	}

	return listeners, nil
}

// Increment helpers for listeners
func (d *SQLDatabase) AddListenerBytes(listenerID string, sent, recv int64) error {
	query := `UPDATE listeners SET bytes_sent = bytes_sent + $1, bytes_recv = bytes_recv + $2, updated_at = CURRENT_TIMESTAMP WHERE id = $3`
	_, err := d.db.Exec(query, sent, recv, listenerID)
	return err
}

func (d *SQLDatabase) AddListenerConnections(listenerID string, delta int) error {
	query := `UPDATE listeners SET connections = connections + $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`
	_, err := d.db.Exec(query, delta, listenerID)
	return err
}

// MarkStaleListenersClosed marks 'open' listeners as 'closed' if UpdatedAt older than threshold
func (d *SQLDatabase) MarkStaleListenersClosed(age time.Duration) error {
	cutoff := time.Now().Add(-age)
	query := `UPDATE listeners SET status = 'closed', updated_at = CURRENT_TIMESTAMP WHERE status = 'open' AND updated_at <= $1`
	_, err := d.db.Exec(query, cutoff)
	return err
}
