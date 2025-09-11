package database

import (
	"fmt"
	"strings"
)

// Migrate creates database tables
func (d *SQLDatabase) Migrate() error {
	var queries []string

	switch d.config.Type {
	case "sqlite":
		queries = d.getSQLiteMigrations()
	case "postgres":
		queries = d.getPostgresMigrations()
	default:
		return fmt.Errorf("unsupported database type: %s", d.config.Type)
	}

	for _, query := range queries {
		if _, err := d.db.Exec(query); err != nil {
			// Ignore "duplicate column" errors for ALTER TABLE statements
			if strings.Contains(query, "ALTER TABLE") &&
				(strings.Contains(err.Error(), "duplicate column") ||
					strings.Contains(err.Error(), "already exists") ||
					strings.Contains(err.Error(), "no such column")) {
				// Column already exists or doesn't exist, continue
				continue
			}
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	return nil
}

func (d *SQLDatabase) getSQLiteMigrations() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			password TEXT NOT NULL,
			email TEXT DEFAULT '',
			display_name TEXT DEFAULT '',
			is_admin BOOLEAN DEFAULT FALSE,
			addresses TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			username TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME NOT NULL,
			ip_address TEXT,
			FOREIGN KEY (username) REFERENCES users(username) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS tunnels (
			id TEXT PRIMARY KEY,
			username TEXT NOT NULL,
			local_port INTEGER NOT NULL,
			local_host TEXT DEFAULT '0.0.0.0',
			remote_port INTEGER NOT NULL,
			remote_host TEXT DEFAULT '127.0.0.1',
			status TEXT DEFAULT 'active',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			bytes_sent INTEGER DEFAULT 0,
			bytes_recv INTEGER DEFAULT 0,
			connections INTEGER DEFAULT 0,
			FOREIGN KEY (username) REFERENCES users(username) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS connections (
			id TEXT PRIMARY KEY,
			tunnel_id TEXT NOT NULL,
			client_ip TEXT,
			start_time DATETIME DEFAULT CURRENT_TIMESTAMP,
			end_time DATETIME,
			bytes_sent INTEGER DEFAULT 0,
			bytes_recv INTEGER DEFAULT 0,
			status TEXT DEFAULT 'active',
			FOREIGN KEY (tunnel_id) REFERENCES tunnels(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_username ON sessions(username)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at)`,
		`CREATE INDEX IF NOT EXISTS idx_tunnels_username ON tunnels(username)`,
		`CREATE INDEX IF NOT EXISTS idx_tunnels_status ON tunnels(status)`,
		`CREATE INDEX IF NOT EXISTS idx_connections_tunnel ON connections(tunnel_id)`,
		`CREATE INDEX IF NOT EXISTS idx_connections_status ON connections(status)`,
		`CREATE TABLE IF NOT EXISTS listeners (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			username TEXT NOT NULL,
			port INTEGER NOT NULL UNIQUE,
			mode TEXT NOT NULL DEFAULT 'sink',
			target_url TEXT,
			response TEXT,
			use_tls BOOLEAN DEFAULT 1,
			status TEXT NOT NULL DEFAULT 'open',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			bytes_sent INTEGER DEFAULT 0,
			bytes_recv INTEGER DEFAULT 0,
			connections INTEGER DEFAULT 0,
			FOREIGN KEY (username) REFERENCES users(username) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_listeners_username ON listeners(username)`,
		`CREATE INDEX IF NOT EXISTS idx_listeners_status ON listeners(status)`,
		`CREATE INDEX IF NOT EXISTS idx_listeners_port ON listeners(port)`,

		// Multicast tunnels (SQLite)
		`CREATE TABLE IF NOT EXISTS multicast_tunnels (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			owner_username TEXT NOT NULL,
			port INTEGER NOT NULL UNIQUE,
			mode TEXT NOT NULL DEFAULT 'webhook',
			enabled BOOLEAN DEFAULT 0,
			visible BOOLEAN DEFAULT 1,
			use_tls BOOLEAN DEFAULT 1,
			status TEXT NOT NULL DEFAULT 'closed',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			bytes_sent INTEGER DEFAULT 0,
			bytes_recv INTEGER DEFAULT 0,
			connections INTEGER DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_multicast_tunnels_port ON multicast_tunnels(port)`,
		`CREATE INDEX IF NOT EXISTS idx_multicast_tunnels_visible ON multicast_tunnels(visible)`,

		// Add use_tls column to existing listeners table if it doesn't exist (SQLite)
		`ALTER TABLE listeners ADD COLUMN use_tls BOOLEAN DEFAULT 1`,
		// Add name column to existing listeners table if it doesn't exist (SQLite)
		`ALTER TABLE listeners ADD COLUMN name TEXT DEFAULT ''`,

		// Create user_tokens table
		`CREATE TABLE IF NOT EXISTS user_tokens (
			id TEXT PRIMARY KEY,
			username TEXT NOT NULL,
			name TEXT NOT NULL,
			token TEXT NOT NULL UNIQUE,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_used DATETIME,
			expires_at DATETIME,
			FOREIGN KEY (username) REFERENCES users(username) ON DELETE CASCADE
		)`,

		// Add email and display_name columns to users table if they don't exist (SQLite)
		`ALTER TABLE users ADD COLUMN email TEXT DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN display_name TEXT DEFAULT ''`,

		// Create settings table for configuration
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Create security webhooks table (SQLite)
		`CREATE TABLE IF NOT EXISTS security_webhooks (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				url TEXT NOT NULL,
				type TEXT NOT NULL,
				enabled BOOLEAN DEFAULT 1,
				description TEXT DEFAULT '',
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
			)`,
		`CREATE INDEX IF NOT EXISTS idx_security_webhooks_enabled ON security_webhooks(enabled)`,

		// Create security events table (SQLite)
		`CREATE TABLE IF NOT EXISTS security_events (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				type TEXT NOT NULL,
				severity TEXT NOT NULL,
				username TEXT,
				ip TEXT,
				message TEXT,
				at DATETIME DEFAULT CURRENT_TIMESTAMP,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP
			)`,

		// Create port reservations table
		`CREATE TABLE IF NOT EXISTS port_reservations (
			id TEXT PRIMARY KEY,
			username TEXT NOT NULL,
			start_port INTEGER NOT NULL,
			end_port INTEGER NOT NULL,
			description TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (username) REFERENCES users(username) ON DELETE CASCADE
		)`,

		// Create indexes for port reservations
		`CREATE INDEX IF NOT EXISTS idx_port_reservations_ports ON port_reservations(start_port, end_port)`,
		`CREATE INDEX IF NOT EXISTS idx_port_reservations_username ON port_reservations(username)`,

		// Set default reserved ports threshold
		`INSERT OR IGNORE INTO settings (key, value, created_at, updated_at)
		 VALUES ('reserved_ports_threshold', '10000', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,

		// Create user limits table
		`CREATE TABLE IF NOT EXISTS user_limits (
			username TEXT PRIMARY KEY,
			max_tunnels INTEGER DEFAULT NULL,
			max_listeners INTEGER DEFAULT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (username) REFERENCES users(username) ON DELETE CASCADE
		)`,

		// Set default global limits
		`INSERT OR IGNORE INTO settings (key, value, created_at, updated_at)
		 VALUES ('default_max_tunnels', '10', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		`INSERT OR IGNORE INTO settings (key, value, created_at, updated_at)
		 VALUES ('default_max_listeners', '5', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,

		// Create SSO configuration table
		`CREATE TABLE IF NOT EXISTS sso_config (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			provider TEXT NOT NULL,
			enabled BOOLEAN DEFAULT FALSE,
			config_json TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Create user authentication sources table
		`CREATE TABLE IF NOT EXISTS user_auth_sources (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL,
			auth_source TEXT NOT NULL DEFAULT 'local',
			external_id TEXT,
			provider_data TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (username) REFERENCES users(username) ON DELETE CASCADE,
			UNIQUE(username, auth_source)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_user_auth_sources_username ON user_auth_sources(username)`,
		`CREATE INDEX IF NOT EXISTS idx_user_auth_sources_external_id ON user_auth_sources(external_id)`,

		// Create user_preferences table
		`CREATE TABLE IF NOT EXISTS user_preferences (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL,
			preference_key TEXT NOT NULL,
			preference_value TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (username) REFERENCES users(username) ON DELETE CASCADE,
			UNIQUE(username, preference_key)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_user_preferences_username ON user_preferences(username)`,

		// Create AI providers table
		`CREATE TABLE IF NOT EXISTS ai_providers (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			provider_type TEXT NOT NULL,
			api_key TEXT NOT NULL,
			api_endpoint TEXT,
			model TEXT NOT NULL,
			max_tokens INTEGER DEFAULT 4000,
			temperature REAL DEFAULT 0.7,
			enabled BOOLEAN DEFAULT true,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			created_by TEXT NOT NULL,
			test_status TEXT DEFAULT 'untested',
			test_message TEXT,
			test_at DATETIME
		)`,
		`CREATE INDEX IF NOT EXISTS idx_ai_providers_type ON ai_providers(provider_type)`,
		`CREATE INDEX IF NOT EXISTS idx_ai_providers_enabled ON ai_providers(enabled)`,

		// Create AI listeners table
		`CREATE TABLE IF NOT EXISTS ai_listeners (
			id TEXT PRIMARY KEY,
			listener_id TEXT NOT NULL,
			ai_provider_id TEXT NOT NULL,
			openapi_spec TEXT,
			system_prompt TEXT,
			conversation_thread TEXT,
			generated_responses TEXT,
			last_generated_at DATETIME,
			generation_status TEXT DEFAULT 'pending',
			generation_error TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (listener_id) REFERENCES listeners(id) ON DELETE CASCADE,
			FOREIGN KEY (ai_provider_id) REFERENCES ai_providers(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_ai_listeners_listener_id ON ai_listeners(listener_id)`,
		`CREATE INDEX IF NOT EXISTS idx_ai_listeners_provider_id ON ai_listeners(ai_provider_id)`,

		// Create AI response versions table for iterative refinement
		`CREATE TABLE IF NOT EXISTS ai_response_versions (
			id TEXT PRIMARY KEY,
			ai_listener_id TEXT NOT NULL,
			version_number INTEGER NOT NULL,
			openapi_spec TEXT NOT NULL,
			system_prompt TEXT,
			user_instructions TEXT,
			generated_responses TEXT NOT NULL,
			generation_status TEXT DEFAULT 'pending',
			generation_error TEXT,
			is_active BOOLEAN DEFAULT FALSE,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (ai_listener_id) REFERENCES ai_listeners(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_ai_response_versions_listener_id ON ai_response_versions(ai_listener_id)`,
		`CREATE INDEX IF NOT EXISTS idx_ai_response_versions_version ON ai_response_versions(ai_listener_id, version_number)`,
		`CREATE INDEX IF NOT EXISTS idx_ai_response_versions_active ON ai_response_versions(ai_listener_id, is_active)`,
	}
}

func (d *SQLDatabase) getPostgresMigrations() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			username VARCHAR(255) UNIQUE NOT NULL,
			password VARCHAR(255) NOT NULL,
			email VARCHAR(255) DEFAULT '',
			display_name VARCHAR(255) DEFAULT '',
			is_admin BOOLEAN DEFAULT FALSE,
			addresses TEXT DEFAULT '',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id VARCHAR(255) PRIMARY KEY,
			username VARCHAR(255) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			expires_at TIMESTAMP NOT NULL,
			ip_address INET,
			FOREIGN KEY (username) REFERENCES users(username) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS tunnels (
			id VARCHAR(255) PRIMARY KEY,
			username VARCHAR(255) NOT NULL,
			local_port INTEGER NOT NULL,
			local_host VARCHAR(255) DEFAULT '0.0.0.0',
			remote_port INTEGER NOT NULL,
			remote_host VARCHAR(255) DEFAULT '127.0.0.1',
			status VARCHAR(50) DEFAULT 'active',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			bytes_sent BIGINT DEFAULT 0,
			bytes_recv BIGINT DEFAULT 0,
			connections INTEGER DEFAULT 0,
			FOREIGN KEY (username) REFERENCES users(username) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS connections (
			id VARCHAR(255) PRIMARY KEY,
			tunnel_id VARCHAR(255) NOT NULL,
			client_ip INET,
			start_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			end_time TIMESTAMP,
			bytes_sent BIGINT DEFAULT 0,
			bytes_recv BIGINT DEFAULT 0,
			status VARCHAR(50) DEFAULT 'active',
			FOREIGN KEY (tunnel_id) REFERENCES tunnels(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_username ON sessions(username)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at)`,
		`CREATE INDEX IF NOT EXISTS idx_tunnels_username ON tunnels(username)`,
		`CREATE INDEX IF NOT EXISTS idx_tunnels_status ON tunnels(status)`,
		`CREATE INDEX IF NOT EXISTS idx_connections_tunnel ON connections(tunnel_id)`,
		`CREATE INDEX IF NOT EXISTS idx_connections_status ON connections(status)`,
		`CREATE TABLE IF NOT EXISTS listeners (
			id VARCHAR(255) PRIMARY KEY,
			name VARCHAR(255) NOT NULL DEFAULT '',
			username VARCHAR(255) NOT NULL,
			port INTEGER NOT NULL UNIQUE,
			mode VARCHAR(50) NOT NULL DEFAULT 'sink',
			target_url TEXT,
			response TEXT,
			use_tls BOOLEAN DEFAULT TRUE,
			status VARCHAR(50) NOT NULL DEFAULT 'open',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			bytes_sent BIGINT DEFAULT 0,
			bytes_recv BIGINT DEFAULT 0,
			connections INTEGER DEFAULT 0,
			FOREIGN KEY (username) REFERENCES users(username) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_listeners_username ON listeners(username)`,
		`CREATE INDEX IF NOT EXISTS idx_listeners_status ON listeners(status)`,
		`CREATE INDEX IF NOT EXISTS idx_listeners_port ON listeners(port)`,

		// Multicast tunnels (PostgreSQL)
		`CREATE TABLE IF NOT EXISTS multicast_tunnels (
			id VARCHAR(255) PRIMARY KEY,
			name VARCHAR(255) NOT NULL DEFAULT '',
			owner_username VARCHAR(255) NOT NULL,
			port INTEGER NOT NULL UNIQUE,
			mode VARCHAR(50) NOT NULL DEFAULT 'webhook',
			enabled BOOLEAN DEFAULT FALSE,
			visible BOOLEAN DEFAULT TRUE,
			use_tls BOOLEAN DEFAULT TRUE,
			status VARCHAR(50) NOT NULL DEFAULT 'closed',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			bytes_sent BIGINT DEFAULT 0,
			bytes_recv BIGINT DEFAULT 0,
			connections INTEGER DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_multicast_tunnels_port ON multicast_tunnels(port)`,
		`CREATE INDEX IF NOT EXISTS idx_multicast_tunnels_visible ON multicast_tunnels(visible)`,

		// Create security events table (PostgreSQL)
		`CREATE TABLE IF NOT EXISTS security_events (
				id SERIAL PRIMARY KEY,
				type VARCHAR(50) NOT NULL,
				severity VARCHAR(20) NOT NULL,
				username VARCHAR(255),
				ip VARCHAR(255),
				message TEXT,
				at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)`,

		// Create security webhooks table (PostgreSQL)
		`CREATE TABLE IF NOT EXISTS security_webhooks (
				id SERIAL PRIMARY KEY,
				url TEXT NOT NULL,
				type VARCHAR(50) NOT NULL,
				enabled BOOLEAN DEFAULT TRUE,
				description TEXT DEFAULT '',
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)`,
		`CREATE INDEX IF NOT EXISTS idx_security_webhooks_enabled ON security_webhooks(enabled)`,

		// Add use_tls column to existing listeners table if it doesn't exist (PostgreSQL)
		`ALTER TABLE listeners ADD COLUMN IF NOT EXISTS use_tls BOOLEAN DEFAULT TRUE`,
		// Add name column to existing listeners table if it doesn't exist (PostgreSQL)
		`ALTER TABLE listeners ADD COLUMN IF NOT EXISTS name VARCHAR(255) DEFAULT ''`,

		// Create settings table for configuration (PostgreSQL)
		`CREATE TABLE IF NOT EXISTS settings (
			key VARCHAR(255) PRIMARY KEY,
			value TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Create port reservations table (PostgreSQL)
		`CREATE TABLE IF NOT EXISTS port_reservations (
			id VARCHAR(255) PRIMARY KEY,
			username VARCHAR(255) NOT NULL,
			start_port INTEGER NOT NULL,
			end_port INTEGER NOT NULL,
			description TEXT DEFAULT '',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (username) REFERENCES users(username) ON DELETE CASCADE
		)`,

		// Create indexes for port reservations (PostgreSQL)
		`CREATE INDEX IF NOT EXISTS idx_port_reservations_ports ON port_reservations(start_port, end_port)`,
		`CREATE INDEX IF NOT EXISTS idx_port_reservations_username ON port_reservations(username)`,

		// Set default reserved ports threshold (PostgreSQL)
		`INSERT INTO settings (key, value, created_at, updated_at)
		 VALUES ('reserved_ports_threshold', '10000', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		 ON CONFLICT (key) DO NOTHING`,

		// Create user limits table (PostgreSQL)
		`CREATE TABLE IF NOT EXISTS user_limits (
			username VARCHAR(255) PRIMARY KEY,
			max_tunnels INTEGER DEFAULT NULL,
			max_listeners INTEGER DEFAULT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (username) REFERENCES users(username) ON DELETE CASCADE
		)`,

		// Set default global limits (PostgreSQL)
		`INSERT INTO settings (key, value, created_at, updated_at)
		 VALUES ('default_max_tunnels', '10', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		 ON CONFLICT (key) DO NOTHING`,
		`INSERT INTO settings (key, value, created_at, updated_at)
		 VALUES ('default_max_listeners', '5', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		 ON CONFLICT (key) DO NOTHING`,

		// Create SSO configuration table (PostgreSQL)
		`CREATE TABLE IF NOT EXISTS sso_config (
			id SERIAL PRIMARY KEY,
			provider VARCHAR(50) NOT NULL,
			enabled BOOLEAN DEFAULT FALSE,
			config_json TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Create user authentication sources table (PostgreSQL)
		`CREATE TABLE IF NOT EXISTS user_auth_sources (
			id SERIAL PRIMARY KEY,
			username VARCHAR(255) NOT NULL,
			auth_source VARCHAR(50) NOT NULL DEFAULT 'local',
			external_id VARCHAR(255),
			provider_data TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (username) REFERENCES users(username) ON DELETE CASCADE,
			UNIQUE(username, auth_source)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_user_auth_sources_username ON user_auth_sources(username)`,
		`CREATE INDEX IF NOT EXISTS idx_user_auth_sources_external_id ON user_auth_sources(external_id)`,

		// Create user_tokens table
		`CREATE TABLE IF NOT EXISTS user_tokens (
			id VARCHAR(255) PRIMARY KEY,
			username VARCHAR(255) NOT NULL,
			name VARCHAR(255) NOT NULL,
			token VARCHAR(255) NOT NULL UNIQUE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			last_used TIMESTAMP,
			expires_at TIMESTAMP,
			FOREIGN KEY (username) REFERENCES users(username) ON DELETE CASCADE
		)`,

		// Add email and display_name columns to users table if they don't exist (PostgreSQL)
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS email VARCHAR(255) DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS display_name VARCHAR(255) DEFAULT ''`,

		// Create user_preferences table (PostgreSQL)
		`CREATE TABLE IF NOT EXISTS user_preferences (
			id SERIAL PRIMARY KEY,
			username VARCHAR(255) NOT NULL,
			preference_key VARCHAR(255) NOT NULL,
			preference_value TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (username) REFERENCES users(username) ON DELETE CASCADE,
			UNIQUE(username, preference_key)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_user_preferences_username ON user_preferences(username)`,

		// Create AI providers table (PostgreSQL)
		`CREATE TABLE IF NOT EXISTS ai_providers (
			id VARCHAR(255) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			provider_type VARCHAR(50) NOT NULL,
			api_key TEXT NOT NULL,
			api_endpoint TEXT,
			model VARCHAR(255) NOT NULL,
			max_tokens INTEGER DEFAULT 4000,
			temperature REAL DEFAULT 0.7,
			enabled BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			created_by VARCHAR(255) NOT NULL,
			test_status VARCHAR(50) DEFAULT 'untested',
			test_message TEXT,
			test_at TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_ai_providers_type ON ai_providers(provider_type)`,
		`CREATE INDEX IF NOT EXISTS idx_ai_providers_enabled ON ai_providers(enabled)`,

		// Create AI listeners table (PostgreSQL)
		`CREATE TABLE IF NOT EXISTS ai_listeners (
			id VARCHAR(255) PRIMARY KEY,
			listener_id VARCHAR(255) NOT NULL,
			ai_provider_id VARCHAR(255) NOT NULL,
			openapi_spec TEXT,
			system_prompt TEXT,
			conversation_thread TEXT,
			generated_responses TEXT,
			last_generated_at TIMESTAMP,
			generation_status VARCHAR(50) DEFAULT 'pending',
			generation_error TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (listener_id) REFERENCES listeners(id) ON DELETE CASCADE,
			FOREIGN KEY (ai_provider_id) REFERENCES ai_providers(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_ai_listeners_listener_id ON ai_listeners(listener_id)`,
		`CREATE INDEX IF NOT EXISTS idx_ai_listeners_provider_id ON ai_listeners(ai_provider_id)`,

		// Create AI response versions table for iterative refinement (PostgreSQL)
		`CREATE TABLE IF NOT EXISTS ai_response_versions (
			id VARCHAR(255) PRIMARY KEY,
			ai_listener_id VARCHAR(255) NOT NULL,
			version_number INTEGER NOT NULL,
			openapi_spec TEXT NOT NULL,
			system_prompt TEXT,
			user_instructions TEXT,
			generated_responses TEXT NOT NULL,
			generation_status VARCHAR(50) DEFAULT 'pending',
			generation_error TEXT,
			is_active BOOLEAN DEFAULT FALSE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (ai_listener_id) REFERENCES ai_listeners(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_ai_response_versions_listener_id ON ai_response_versions(ai_listener_id)`,
		`CREATE INDEX IF NOT EXISTS idx_ai_response_versions_version ON ai_response_versions(ai_listener_id, version_number)`,
		`CREATE INDEX IF NOT EXISTS idx_ai_response_versions_active ON ai_response_versions(ai_listener_id, is_active)`,
	}
}
