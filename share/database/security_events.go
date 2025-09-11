package database

import (
	"database/sql"
	"fmt"
	"time"
)

// SecurityEventLog is a persisted security event row
type SecurityEventLog struct {
	ID        int       `db:"id" json:"id"`
	Type      string    `db:"type" json:"type"`
	Severity  string    `db:"severity" json:"severity"`
	Username  string    `db:"username" json:"username"`
	IP        string    `db:"ip" json:"ip"`
	Message   string    `db:"message" json:"message"`
	At        time.Time `db:"at" json:"at"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// InsertSecurityEvent writes a new security event to the database
func (d *SQLDatabase) InsertSecurityEvent(ev *SecurityEventLog) error {
	now := time.Now()
	if ev.At.IsZero() {
		ev.At = now
	}
	ev.CreatedAt = now
	var err error
	if d.config.Type == "postgres" {
		_, err = d.db.Exec(`INSERT INTO security_events (type, severity, username, ip, message, at, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`, ev.Type, ev.Severity, ev.Username, ev.IP, ev.Message, ev.At, ev.CreatedAt)
	} else {
		_, err = d.db.Exec(`INSERT INTO security_events (type, severity, username, ip, message, at, created_at) VALUES (?,?,?,?,?,?,?)`, ev.Type, ev.Severity, ev.Username, ev.IP, ev.Message, ev.At, ev.CreatedAt)
	}
	if err != nil {
		return fmt.Errorf("failed to insert security event: %w", err)
	}
	return nil
}

// ListSecurityEvents returns the most recent events up to limit
func (d *SQLDatabase) ListSecurityEvents(limit int) ([]SecurityEventLog, error) {
	if limit <= 0 || limit > 10000 {
		limit = 100
	}
	var rows []SecurityEventLog
	var err error
	if d.config.Type == "postgres" {
		err = d.db.Select(&rows, `SELECT id, type, severity, username, ip, message, at, created_at FROM security_events ORDER BY at DESC LIMIT $1`, limit)
	} else {
		err = d.db.Select(&rows, `SELECT id, type, severity, username, ip, message, at, created_at FROM security_events ORDER BY at DESC LIMIT ?`, limit)
	}
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	return rows, nil
}
