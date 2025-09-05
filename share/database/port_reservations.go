package database

import (
	"database/sql"
	"fmt"
	"time"
)

// CreatePortReservation creates a new port reservation
func (d *SQLDatabase) CreatePortReservation(reservation *PortReservation) error {
	reservation.ID = fmt.Sprintf("port-res-%d-%d", time.Now().Unix(), time.Now().UnixNano())
	reservation.CreatedAt = time.Now()
	reservation.UpdatedAt = time.Now()

	query := `INSERT INTO port_reservations (id, username, start_port, end_port, description, created_at, updated_at) 
			  VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := d.db.Exec(query, reservation.ID, reservation.Username, reservation.StartPort,
		reservation.EndPort, reservation.Description, reservation.CreatedAt, reservation.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create port reservation: %w", err)
	}

	return nil
}

// GetPortReservation retrieves a port reservation by ID
func (d *SQLDatabase) GetPortReservation(id string) (*PortReservation, error) {
	reservation := &PortReservation{}
	query := `SELECT id, username, start_port, end_port, description, created_at, updated_at 
			  FROM port_reservations WHERE id = $1`

	err := d.db.Get(reservation, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("port reservation not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get port reservation: %w", err)
	}

	return reservation, nil
}

// ListPortReservations retrieves all port reservations
func (d *SQLDatabase) ListPortReservations() ([]*PortReservation, error) {
	var reservations []*PortReservation
	query := `SELECT id, username, start_port, end_port, description, created_at, updated_at 
			  FROM port_reservations ORDER BY start_port`

	err := d.db.Select(&reservations, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list port reservations: %w", err)
	}

	return reservations, nil
}

// ListUserPortReservations retrieves port reservations for a specific user
func (d *SQLDatabase) ListUserPortReservations(username string) ([]*PortReservation, error) {
	var reservations []*PortReservation
	query := `SELECT id, username, start_port, end_port, description, created_at, updated_at 
			  FROM port_reservations WHERE username = $1 ORDER BY start_port`

	err := d.db.Select(&reservations, query, username)
	if err != nil {
		return nil, fmt.Errorf("failed to list user port reservations: %w", err)
	}

	return reservations, nil
}

// DeletePortReservation deletes a port reservation by ID
func (d *SQLDatabase) DeletePortReservation(id string) error {
	query := `DELETE FROM port_reservations WHERE id = $1`

	result, err := d.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete port reservation: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("port reservation not found: %s", id)
	}

	return nil
}

// GetReservedPortsThreshold gets the reserved ports threshold from settings
func (d *SQLDatabase) GetReservedPortsThreshold() (int, error) {
	var threshold int
	query := `SELECT value FROM settings WHERE key = 'reserved_ports_threshold'`

	err := d.db.Get(&threshold, query)
	if err != nil {
		if err == sql.ErrNoRows {
			// Default threshold is 10000
			return 10000, nil
		}
		return 0, fmt.Errorf("failed to get reserved ports threshold: %w", err)
	}

	return threshold, nil
}

// SetReservedPortsThreshold sets the reserved ports threshold in settings
func (d *SQLDatabase) SetReservedPortsThreshold(threshold int) error {
	query := `INSERT INTO settings (key, value, updated_at) VALUES ('reserved_ports_threshold', $1, $2)
			  ON CONFLICT (key) DO UPDATE SET value = $1, updated_at = $2`

	_, err := d.db.Exec(query, threshold, time.Now())
	if err != nil {
		return fmt.Errorf("failed to set reserved ports threshold: %w", err)
	}

	return nil
}

// IsPortReserved checks if a port is reserved and if the user can use it
func (d *SQLDatabase) IsPortReserved(port int, username string) (bool, error) {
	// Get the reserved ports threshold
	threshold, err := d.GetReservedPortsThreshold()
	if err != nil {
		return false, err
	}

	// If port is above threshold, it's not reserved
	if port >= threshold {
		return false, nil
	}

	// Check if user has a specific reservation for this port
	var count int
	query := `SELECT COUNT(*) FROM port_reservations 
			  WHERE username = $1 AND start_port <= $2 AND end_port >= $2`

	err = d.db.Get(&count, query, username, port)
	if err != nil {
		return false, fmt.Errorf("failed to check port reservation: %w", err)
	}

	// If user has a reservation covering this port, they can use it
	if count > 0 {
		return false, nil
	}

	// Port is reserved and user doesn't have access
	return true, nil
}
