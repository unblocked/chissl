package database

import (
	"fmt"
	"time"
)

// MulticastTunnel represents a persistent TLS webhook/broadcast endpoint
type MulticastTunnel struct {
	ID          string    `db:"id" json:"id"`
	Name        string    `db:"name" json:"name"`
	Owner       string    `db:"owner_username" json:"owner_username"`
	Port        int       `db:"port" json:"port"`
	Mode        string    `db:"mode" json:"mode"` // "webhook" | "bidirectional" (phase 2)
	Enabled     bool      `db:"enabled" json:"enabled"`
	Visible     bool      `db:"visible" json:"visible"`
	UseTLS      bool      `db:"use_tls" json:"use_tls"`
	Status      string    `db:"status" json:"status"` // "open", "closed", "error"
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
	BytesSent   int64     `db:"bytes_sent" json:"bytes_sent"`
	BytesRecv   int64     `db:"bytes_recv" json:"bytes_recv"`
	Connections int       `db:"connections" json:"connections"`
}

// CreateMulticastTunnel inserts a new multicast tunnel
func (d *SQLDatabase) CreateMulticastTunnel(mt *MulticastTunnel) error {
	mt.CreatedAt = time.Now()
	mt.UpdatedAt = time.Now()
	if mt.UseTLS == false {
		// Enforce TLS always on
		mt.UseTLS = true
	}
	if mt.Mode == "" {
		mt.Mode = "webhook"
	}
	if mt.Status == "" {
		mt.Status = "closed"
	}
	q := `INSERT INTO multicast_tunnels (id, name, owner_username, port, mode, enabled, visible, use_tls, status, created_at, updated_at, bytes_sent, bytes_recv, connections)
	      VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`
	_, err := d.db.Exec(q, mt.ID, mt.Name, mt.Owner, mt.Port, mt.Mode, mt.Enabled, mt.Visible, mt.UseTLS, mt.Status, mt.CreatedAt, mt.UpdatedAt, mt.BytesSent, mt.BytesRecv, mt.Connections)
	if err != nil {
		return fmt.Errorf("failed to create multicast tunnel: %w", err)
	}
	return nil
}

// UpdateMulticastTunnel updates fields on a multicast tunnel
func (d *SQLDatabase) UpdateMulticastTunnel(mt *MulticastTunnel) error {
	mt.UpdatedAt = time.Now()
	q := `UPDATE multicast_tunnels SET
		name = COALESCE(NULLIF($1, ''), name),
		port = COALESCE(NULLIF($2, 0), port),
		mode = COALESCE(NULLIF($3, ''), mode),
		enabled = COALESCE($4, enabled),
		visible = COALESCE($5, visible),
		use_tls = COALESCE($6, use_tls),
		status = COALESCE(NULLIF($7, ''), status),
		updated_at = $8,
		bytes_sent = COALESCE($9, bytes_sent),
		bytes_recv = COALESCE($10, bytes_recv),
		connections = COALESCE($11, connections)
		WHERE id = $12`
	res, err := d.db.Exec(q, mt.Name, mt.Port, mt.Mode, mt.Enabled, mt.Visible, mt.UseTLS, mt.Status, mt.UpdatedAt, mt.BytesSent, mt.BytesRecv, mt.Connections, mt.ID)
	if err != nil {
		return fmt.Errorf("failed to update multicast tunnel: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("multicast tunnel not found: %s", mt.ID)
	}
	return nil
}

// DeleteMulticastTunnel removes a multicast tunnel by ID
func (d *SQLDatabase) DeleteMulticastTunnel(id string) error {
	q := `DELETE FROM multicast_tunnels WHERE id = $1`
	_, err := d.db.Exec(q, id)
	return err
}

// GetMulticastTunnel retrieves a multicast tunnel by ID
func (d *SQLDatabase) GetMulticastTunnel(id string) (*MulticastTunnel, error) {
	mt := &MulticastTunnel{}
	q := `SELECT id, name, owner_username, port, mode, enabled, visible, use_tls, status, created_at, updated_at, bytes_sent, bytes_recv, connections FROM multicast_tunnels WHERE id = $1`
	if err := d.db.Get(mt, q, id); err != nil {
		return nil, fmt.Errorf("failed to get multicast tunnel: %w", err)
	}
	return mt, nil
}

// ListMulticastTunnels lists all multicast tunnels
func (d *SQLDatabase) ListMulticastTunnels() ([]*MulticastTunnel, error) {
	s := make([]*MulticastTunnel, 0)
	q := `SELECT id, name, owner_username, port, mode, enabled, visible, use_tls, status, created_at, updated_at, bytes_sent, bytes_recv, connections FROM multicast_tunnels ORDER BY created_at DESC`
	if err := d.db.Select(&s, q); err != nil {
		return nil, fmt.Errorf("failed to list multicast tunnels: %w", err)
	}
	return s, nil
}

// ListPublicMulticastTunnels lists visible multicast tunnels
func (d *SQLDatabase) ListPublicMulticastTunnels() ([]*MulticastTunnel, error) {
	s := make([]*MulticastTunnel, 0)
	q := `SELECT id, name, owner_username, port, mode, enabled, visible, use_tls, status, created_at, updated_at, bytes_sent, bytes_recv, connections FROM multicast_tunnels WHERE visible = 1 ORDER BY created_at DESC`
	if err := d.db.Select(&s, q); err != nil {
		return nil, fmt.Errorf("failed to list public multicast tunnels: %w", err)
	}
	return s, nil
}

// AddMulticastBytes increments byte counters
func (d *SQLDatabase) AddMulticastBytes(id string, sent, recv int64) error {
	q := `UPDATE multicast_tunnels SET bytes_sent = bytes_sent + $1, bytes_recv = bytes_recv + $2, updated_at = CURRENT_TIMESTAMP WHERE id = $3`
	_, err := d.db.Exec(q, sent, recv, id)
	return err
}

// AddMulticastConnections increments active connection count
func (d *SQLDatabase) AddMulticastConnections(id string, delta int) error {
	q := `UPDATE multicast_tunnels SET connections = connections + $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`
	_, err := d.db.Exec(q, delta, id)
	return err
}
