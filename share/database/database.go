package database

import (
	"fmt"
	"log"
	"regexp"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Type     string // "sqlite" or "postgres"
	Host     string
	Port     int
	Database string
	Username string
	Password string
	SSLMode  string
	FilePath string // for SQLite
}

// Database interface defines database operations
type Database interface {
	Connect() error
	Close() error
	Migrate() error
	GetUser(username string) (*User, error)
	CreateUser(user *User) error
	UpdateUser(user *User) error
	DeleteUser(username string) error
	ListUsers() ([]*User, error)
	CreateSession(session *Session) error
	GetSession(sessionID string) (*Session, error)
	DeleteSession(sessionID string) error
	CreateTunnel(tunnel *Tunnel) error
	UpdateTunnel(tunnel *Tunnel) error
	DeleteTunnel(tunnelID string) error
	GetTunnel(tunnelID string) (*Tunnel, error)
	ListTunnels() ([]*Tunnel, error)
	ListActiveTunnels() ([]*Tunnel, error)
	CreateConnection(conn *Connection) error
	UpdateConnection(conn *Connection) error
	ListConnections(tunnelID string) ([]*Connection, error)
	GetStats() (*Stats, error)
	GetUserStats(username string) (*Stats, error)
	AddTunnelBytes(tunnelID string, sent, recv int64) error
	AddTunnelConnections(tunnelID string, delta int) error
	MarkStaleTunnelsClosed(age time.Duration) error
	// Listener management
	CreateListener(listener *Listener) error
	UpdateListener(listener *Listener) error
	DeleteListener(listenerID string) error
	GetListener(listenerID string) (*Listener, error)
	ListListeners() ([]*Listener, error)
	ListActiveListeners() ([]*Listener, error)
	AddListenerBytes(listenerID string, sent, recv int64) error
	AddListenerConnections(listenerID string, delta int) error
	MarkStaleListenersClosed(age time.Duration) error

	// User token management
	CreateUserToken(token *UserToken) error
	GetUserToken(id string) (*UserToken, error)
	ListUserTokens(username string) ([]*UserToken, error)
	DeleteUserToken(id string) error
	UpdateUserTokenLastUsed(id string, lastUsed time.Time) error
	ValidateUserToken(token string) (*UserToken, error)

	// Port reservation management
	CreatePortReservation(reservation *PortReservation) error
	GetPortReservation(id string) (*PortReservation, error)
	ListPortReservations() ([]*PortReservation, error)
	ListUserPortReservations(username string) ([]*PortReservation, error)
	DeletePortReservation(id string) error
	GetReservedPortsThreshold() (int, error)
	SetReservedPortsThreshold(threshold int) error
	IsPortReserved(port int, username string) (bool, error)

	// User limits management
	CreateUserLimits(limits *UserLimits) error
	GetUserLimits(username string) (*UserLimits, error)
	UpdateUserLimits(limits *UserLimits) error
	DeleteUserLimits(username string) error
	GetEffectiveUserLimits(username string) (maxTunnels, maxListeners int, err error)
	CheckUserTunnelLimit(username string) (bool, error)
	CheckUserListenerLimit(username string) (bool, error)
	GetSettingInt(key string, defaultValue int) (int, error)
}

// User represents a user in the database
type User struct {
	ID          int       `db:"id" json:"id"`
	Username    string    `db:"username" json:"username"`
	Password    string    `db:"password" json:"password,omitempty"`
	Email       string    `db:"email" json:"email,omitempty"`
	DisplayName string    `db:"display_name" json:"display_name,omitempty"`
	IsAdmin     bool      `db:"is_admin" json:"is_admin"`
	Addresses   string    `db:"addresses" json:"addresses"` // JSON array of regex patterns
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

// UserToken represents an API token for a user
type UserToken struct {
	ID        string     `db:"id" json:"id"`
	Username  string     `db:"username" json:"username"`
	Name      string     `db:"name" json:"name"`
	Token     string     `db:"token" json:"-"` // Don't expose token in JSON
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
	LastUsed  *time.Time `db:"last_used" json:"last_used,omitempty"`
	ExpiresAt *time.Time `db:"expires_at" json:"expires_at,omitempty"`
}

// PortReservation represents a port range reservation for a user
type PortReservation struct {
	ID          string    `db:"id" json:"id"`
	Username    string    `db:"username" json:"username"`
	StartPort   int       `db:"start_port" json:"start_port"`
	EndPort     int       `db:"end_port" json:"end_port"`
	Description string    `db:"description" json:"description"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

// Session represents an active user session
type Session struct {
	ID        string    `db:"id" json:"id"`
	Username  string    `db:"username" json:"username"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	ExpiresAt time.Time `db:"expires_at" json:"expires_at"`
	IPAddress string    `db:"ip_address" json:"ip_address"`
}

// Tunnel represents an active tunnel
type Tunnel struct {
	ID          string    `db:"id" json:"id"`
	Username    string    `db:"username" json:"username"`
	LocalPort   int       `db:"local_port" json:"local_port"`
	LocalHost   string    `db:"local_host" json:"local_host"`
	RemotePort  int       `db:"remote_port" json:"remote_port"`
	RemoteHost  string    `db:"remote_host" json:"remote_host"`
	Status      string    `db:"status" json:"status"` // "active", "inactive", "error"
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
	BytesSent   int64     `db:"bytes_sent" json:"bytes_sent"`
	BytesRecv   int64     `db:"bytes_recv" json:"bytes_recv"`
	Connections int       `db:"connections" json:"connections"`
}

// Connection represents a connection through a tunnel
type Connection struct {
	ID        string     `db:"id" json:"id"`
	TunnelID  string     `db:"tunnel_id" json:"tunnel_id"`
	ClientIP  string     `db:"client_ip" json:"client_ip"`
	StartTime time.Time  `db:"start_time" json:"start_time"`
	EndTime   *time.Time `db:"end_time" json:"end_time,omitempty"`
	BytesSent int64      `db:"bytes_sent" json:"bytes_sent"`
	BytesRecv int64      `db:"bytes_recv" json:"bytes_recv"`
	Status    string     `db:"status" json:"status"` // "active", "closed", "error"
}

// Listener represents a managed HTTP listener/proxy
type Listener struct {
	ID          string    `db:"id" json:"id"`
	Name        string    `db:"name" json:"name"` // human-friendly name
	Username    string    `db:"username" json:"username"`
	Port        int       `db:"port" json:"port"`
	Mode        string    `db:"mode" json:"mode"`                       // "sink" or "proxy"
	TargetURL   string    `db:"target_url" json:"target_url,omitempty"` // for proxy mode
	Response    string    `db:"response" json:"response,omitempty"`     // for sink mode
	UseTLS      bool      `db:"use_tls" json:"use_tls"`                 // whether to use TLS
	Status      string    `db:"status" json:"status"`                   // "open", "closed", "error"
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
	BytesSent   int64     `db:"bytes_sent" json:"bytes_sent"`
	BytesRecv   int64     `db:"bytes_recv" json:"bytes_recv"`
	Connections int       `db:"connections" json:"connections"`
}

// Stats represents system statistics
type Stats struct {
	TotalTunnels     int   `json:"total_tunnels"`
	ActiveTunnels    int   `json:"active_tunnels"`
	TotalListeners   int   `json:"total_listeners"`
	ActiveListeners  int   `json:"active_listeners"`
	TotalUsers       int   `json:"total_users"`
	ActiveSessions   int   `json:"active_sessions"`
	TotalConnections int   `json:"total_connections"`
	TotalBytesSent   int64 `json:"total_bytes_sent"`
	TotalBytesRecv   int64 `json:"total_bytes_recv"`
	UptimeSeconds    int64 `json:"uptime_seconds"`
}

// SQLDatabase implements the Database interface
type SQLDatabase struct {
	db     *sqlx.DB
	config *DatabaseConfig
}

// NewDatabase creates a new database instance
func NewDatabase(config *DatabaseConfig) Database {
	return &SQLDatabase{
		config: config,
	}
}

// Connect establishes database connection
func (d *SQLDatabase) Connect() error {
	var dsn string
	var err error

	switch d.config.Type {
	case "sqlite":
		if d.config.FilePath == "" {
			d.config.FilePath = "./chissl.db"
		}
		dsn = d.config.FilePath
		d.db, err = sqlx.Connect("sqlite3", dsn)
	case "postgres":
		dsn = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			d.config.Host, d.config.Port, d.config.Username, d.config.Password, d.config.Database, d.config.SSLMode)
		d.db, err = sqlx.Connect("postgres", dsn)
	default:
		return fmt.Errorf("unsupported database type: %s", d.config.Type)
	}

	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Test the connection
	if err := d.db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	log.Printf("Connected to %s database successfully", d.config.Type)
	return nil
}

// Close closes the database connection
func (d *SQLDatabase) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

// GetAddressRegexps converts addresses string to regexp slice
func (u *User) GetAddressRegexps() ([]*regexp.Regexp, error) {
	if u.Addresses == "" {
		return []*regexp.Regexp{}, nil
	}

	// For now, assume it's a simple comma-separated list
	// In a full implementation, this would parse JSON
	var regexps []*regexp.Regexp
	if u.Addresses != "" {
		re, err := regexp.Compile(u.Addresses)
		if err != nil {
			return nil, err
		}
		regexps = append(regexps, re)
	}
	return regexps, nil
}

// HasAccess checks if user has access to an address
func (u *User) HasAccess(addr string) bool {
	regexps, err := u.GetAddressRegexps()
	if err != nil {
		return false
	}

	if len(regexps) == 0 {
		return true // No restrictions
	}

	for _, re := range regexps {
		if re.MatchString(addr) {
			return true
		}
	}
	return false
}
