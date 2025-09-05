package database

import (
	"database/sql"
	"fmt"
	"time"
)

// CreateTunnel creates a new tunnel
func (d *SQLDatabase) CreateTunnel(tunnel *Tunnel) error {
	tunnel.CreatedAt = time.Now()
	tunnel.UpdatedAt = time.Now()

	query := `INSERT INTO tunnels (id, username, local_port, local_host, remote_port,
			  remote_host, status, created_at, updated_at, bytes_sent, bytes_recv, connections)
			  VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

	_, err := d.db.Exec(query, tunnel.ID, tunnel.Username, tunnel.LocalPort, tunnel.LocalHost,
		tunnel.RemotePort, tunnel.RemoteHost, tunnel.Status, tunnel.CreatedAt, tunnel.UpdatedAt,
		tunnel.BytesSent, tunnel.BytesRecv, tunnel.Connections)
	if err != nil {
		return fmt.Errorf("failed to create tunnel: %w", err)
	}

	return nil
}

// UpdateTunnel updates an existing tunnel
func (d *SQLDatabase) UpdateTunnel(tunnel *Tunnel) error {
	tunnel.UpdatedAt = time.Now()

	query := `UPDATE tunnels SET status = COALESCE($1, status), updated_at = $2, bytes_sent = COALESCE($3, bytes_sent),
			  bytes_recv = COALESCE($4, bytes_recv), connections = COALESCE($5, connections) WHERE id = $6`

	result, err := d.db.Exec(query, tunnel.Status, tunnel.UpdatedAt, tunnel.BytesSent,
		tunnel.BytesRecv, tunnel.Connections, tunnel.ID)
	if err != nil {
		return fmt.Errorf("failed to update tunnel: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("tunnel not found: %s", tunnel.ID)
	}

	return nil
}

// DeleteTunnel deletes a tunnel by ID
func (d *SQLDatabase) DeleteTunnel(tunnelID string) error {
	query := `DELETE FROM tunnels WHERE id = $1`

	result, err := d.db.Exec(query, tunnelID)
	if err != nil {
		return fmt.Errorf("failed to delete tunnel: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("tunnel not found: %s", tunnelID)
	}

	return nil
}

// GetTunnel retrieves a tunnel by ID
func (d *SQLDatabase) GetTunnel(tunnelID string) (*Tunnel, error) {
	tunnel := &Tunnel{}
	query := `SELECT id, username, local_port, local_host, remote_port, remote_host,
			  status, created_at, updated_at, bytes_sent, bytes_recv, connections
			  FROM tunnels WHERE id = $1`

	err := d.db.Get(tunnel, query, tunnelID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("tunnel not found: %s", tunnelID)
		}
		return nil, fmt.Errorf("failed to get tunnel: %w", err)
	}

	return tunnel, nil
}

// ListTunnels retrieves all tunnels
func (d *SQLDatabase) ListTunnels() ([]*Tunnel, error) {
	var tunnels []*Tunnel
	query := `SELECT id, username, local_port, local_host, remote_port, remote_host,
			  status, created_at, updated_at, bytes_sent, bytes_recv, connections
			  FROM tunnels ORDER BY created_at DESC`

	err := d.db.Select(&tunnels, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list tunnels: %w", err)
	}

	return tunnels, nil
}

// ListActiveTunnels retrieves all active tunnels
func (d *SQLDatabase) ListActiveTunnels() ([]*Tunnel, error) {
	var tunnels []*Tunnel
	query := `SELECT id, username, local_port, local_host, remote_port, remote_host,
			  status, created_at, updated_at, bytes_sent, bytes_recv, connections
			  FROM tunnels WHERE status = 'open' ORDER BY created_at DESC`

	err := d.db.Select(&tunnels, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list active tunnels: %w", err)
	}

	return tunnels, nil
}

// Increment helpers for bytes and connections
func (d *SQLDatabase) AddTunnelBytes(tunnelID string, sent, recv int64) error {
	query := `UPDATE tunnels SET bytes_sent = bytes_sent + $1, bytes_recv = bytes_recv + $2, updated_at = CURRENT_TIMESTAMP WHERE id = $3`
	_, err := d.db.Exec(query, sent, recv, tunnelID)
	return err
}

func (d *SQLDatabase) AddTunnelConnections(tunnelID string, delta int) error {
	query := `UPDATE tunnels SET connections = connections + $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`
	_, err := d.db.Exec(query, delta, tunnelID)
	return err
}

// CreateConnection creates a new connection
func (d *SQLDatabase) CreateConnection(conn *Connection) error {
	conn.StartTime = time.Now()

	query := `INSERT INTO connections (id, tunnel_id, client_ip, start_time, bytes_sent, bytes_recv, status)
			  VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := d.db.Exec(query, conn.ID, conn.TunnelID, conn.ClientIP, conn.StartTime,
		conn.BytesSent, conn.BytesRecv, conn.Status)
	if err != nil {
		return fmt.Errorf("failed to create connection: %w", err)
	}

	return nil
}

// UpdateConnection updates an existing connection
func (d *SQLDatabase) UpdateConnection(conn *Connection) error {
	query := `UPDATE connections SET end_time = $1, bytes_sent = $2, bytes_recv = $3, status = $4
			  WHERE id = $5`

	_, err := d.db.Exec(query, conn.EndTime, conn.BytesSent, conn.BytesRecv, conn.Status, conn.ID)
	if err != nil {
		return fmt.Errorf("failed to update connection: %w", err)
	}

	return nil
}

// ListConnections retrieves connections for a tunnel
func (d *SQLDatabase) ListConnections(tunnelID string) ([]*Connection, error) {
	var connections []*Connection
	query := `SELECT id, tunnel_id, client_ip, start_time, end_time, bytes_sent, bytes_recv, status
			  FROM connections WHERE tunnel_id = $1 ORDER BY start_time DESC`

	err := d.db.Select(&connections, query, tunnelID)
	if err != nil {
		return nil, fmt.Errorf("failed to list connections: %w", err)
	}

	return connections, nil
}

// GetStats retrieves system statistics
func (d *SQLDatabase) GetStats() (*Stats, error) {
	stats := &Stats{}

	// Get total tunnels
	err := d.db.Get(&stats.TotalTunnels, "SELECT COUNT(*) FROM tunnels")
	if err != nil {
		return nil, fmt.Errorf("failed to get total tunnels count: %w", err)
	}

	// Get active tunnels
	err = d.db.Get(&stats.ActiveTunnels, "SELECT COUNT(*) FROM tunnels WHERE status = 'open'")
	if err != nil {
		return nil, fmt.Errorf("failed to get active tunnels count: %w", err)
	}

	// Get total listeners
	err = d.db.Get(&stats.TotalListeners, "SELECT COUNT(*) FROM listeners")
	if err != nil {
		return nil, fmt.Errorf("failed to get total listeners count: %w", err)
	}

	// Get active listeners
	err = d.db.Get(&stats.ActiveListeners, "SELECT COUNT(*) FROM listeners WHERE status = 'open'")
	if err != nil {
		return nil, fmt.Errorf("failed to get active listeners count: %w", err)
	}

	// Get total users
	err = d.db.Get(&stats.TotalUsers, "SELECT COUNT(*) FROM users")
	if err != nil {
		return nil, fmt.Errorf("failed to get user count: %w", err)
	}

	// Get active sessions
	err = d.db.Get(&stats.ActiveSessions, "SELECT COUNT(*) FROM sessions WHERE expires_at > $1", time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to get active sessions count: %w", err)
	}

	// Get total connections
	err = d.db.Get(&stats.TotalConnections, "SELECT COUNT(*) FROM connections")
	if err != nil {
		return nil, fmt.Errorf("failed to get total connections count: %w", err)
	}

	// Get total bytes
	err = d.db.Get(&stats.TotalBytesSent, "SELECT COALESCE(SUM(bytes_sent), 0) FROM tunnels")
	if err != nil {
		return nil, fmt.Errorf("failed to get total bytes sent: %w", err)
	}

	err = d.db.Get(&stats.TotalBytesRecv, "SELECT COALESCE(SUM(bytes_recv), 0) FROM tunnels")
	if err != nil {
		return nil, fmt.Errorf("failed to get total bytes received: %w", err)
	}

	// Uptime would be calculated elsewhere
	stats.UptimeSeconds = 0

	return stats, nil
}

// GetUserStats retrieves statistics for a specific user
func (d *SQLDatabase) GetUserStats(username string) (*Stats, error) {
	stats := &Stats{}

	// Get user's total tunnels
	err := d.db.Get(&stats.TotalTunnels, "SELECT COUNT(*) FROM tunnels WHERE username = $1", username)
	if err != nil {
		return nil, fmt.Errorf("failed to get user total tunnels count: %w", err)
	}

	// Get user's active tunnels
	err = d.db.Get(&stats.ActiveTunnels, "SELECT COUNT(*) FROM tunnels WHERE username = $1 AND status = 'open'", username)
	if err != nil {
		return nil, fmt.Errorf("failed to get user active tunnels count: %w", err)
	}

	// Get user's total listeners
	err = d.db.Get(&stats.TotalListeners, "SELECT COUNT(*) FROM listeners WHERE username = $1", username)
	if err != nil {
		return nil, fmt.Errorf("failed to get user total listeners count: %w", err)
	}

	// Get user's active listeners
	err = d.db.Get(&stats.ActiveListeners, "SELECT COUNT(*) FROM listeners WHERE username = $1 AND status = 'open'", username)
	if err != nil {
		return nil, fmt.Errorf("failed to get user active listeners count: %w", err)
	}

	// User-specific stats don't include total users or sessions
	stats.TotalUsers = 0
	stats.ActiveSessions = 0

	// Get user's total connections
	err = d.db.Get(&stats.TotalConnections, "SELECT COUNT(*) FROM connections c JOIN tunnels t ON c.tunnel_id = t.id WHERE t.username = $1", username)
	if err != nil {
		return nil, fmt.Errorf("failed to get user total connections count: %w", err)
	}

	// Get user's total bytes
	err = d.db.Get(&stats.TotalBytesSent, "SELECT COALESCE(SUM(bytes_sent), 0) FROM tunnels WHERE username = $1", username)
	if err != nil {
		return nil, fmt.Errorf("failed to get user total bytes sent: %w", err)
	}

	err = d.db.Get(&stats.TotalBytesRecv, "SELECT COALESCE(SUM(bytes_recv), 0) FROM tunnels WHERE username = $1", username)
	if err != nil {
		return nil, fmt.Errorf("failed to get user total bytes received: %w", err)
	}

	// Uptime is not user-specific
	stats.UptimeSeconds = 0

	return stats, nil
}

// MarkStaleTunnelsClosed marks 'active' tunnels as 'closed' if UpdatedAt older than threshold.
func (d *SQLDatabase) MarkStaleTunnelsClosed(age time.Duration) error {
	// SQLite and Postgres both understand CURRENT_TIMESTAMP
	// We compare updated_at <= NOW - age
	// For portability across drivers, we compute cutoff here.
	cutoff := time.Now().Add(-age)
	query := `UPDATE tunnels SET status = 'closed', updated_at = CURRENT_TIMESTAMP WHERE status = 'open' AND updated_at <= $1`
	_, err := d.db.Exec(query, cutoff)
	return err
}
