package database

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
)

// Encryption key for API keys (should be loaded from environment)
var encryptionKey = []byte("chissl-ai-provider-encryption-32") // TODO: Load from config/env

// EncryptAPIKey encrypts an API key for secure storage
func EncryptAPIKey(plaintext string) (string, error) {
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptAPIKey decrypts an API key from storage
func DecryptAPIKey(ciphertext string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// CreateAIProvider creates a new AI provider configuration
func (d *SQLDatabase) CreateAIProvider(provider *AIProvider) error {
	if provider.ID == "" {
		provider.ID = uuid.New().String()
	}

	now := time.Now()
	provider.CreatedAt = now
	provider.UpdatedAt = now

	query := `INSERT INTO ai_providers (
		id, name, provider_type, api_key, api_endpoint, model, 
		max_tokens, temperature, enabled, created_at, updated_at, 
		created_by, test_status, test_message, test_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := d.db.Exec(query,
		provider.ID, provider.Name, provider.ProviderType, provider.APIKey,
		provider.APIEndpoint, provider.Model, provider.MaxTokens, provider.Temperature,
		provider.Enabled, provider.CreatedAt, provider.UpdatedAt, provider.CreatedBy,
		provider.TestStatus, provider.TestMessage, provider.TestAt,
	)

	return err
}

// GetAIProvider retrieves an AI provider by ID
func (d *SQLDatabase) GetAIProvider(id string) (*AIProvider, error) {
	query := `SELECT id, name, provider_type, api_key, api_endpoint, model,
		max_tokens, temperature, enabled, created_at, updated_at, created_by,
		test_status, test_message, test_at FROM ai_providers WHERE id = ?`

	provider := &AIProvider{}
	err := d.db.QueryRow(query, id).Scan(
		&provider.ID, &provider.Name, &provider.ProviderType, &provider.APIKey,
		&provider.APIEndpoint, &provider.Model, &provider.MaxTokens, &provider.Temperature,
		&provider.Enabled, &provider.CreatedAt, &provider.UpdatedAt, &provider.CreatedBy,
		&provider.TestStatus, &provider.TestMessage, &provider.TestAt,
	)

	if err != nil {
		return nil, err
	}

	return provider, nil
}

// GetAIProviders retrieves all AI providers
func (d *SQLDatabase) GetAIProviders() ([]*AIProvider, error) {
	query := `SELECT id, name, provider_type, api_key, api_endpoint, model,
		max_tokens, temperature, enabled, created_at, updated_at, created_by,
		test_status, test_message, test_at FROM ai_providers ORDER BY created_at DESC`

	rows, err := d.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var providers []*AIProvider
	for rows.Next() {
		provider := &AIProvider{}
		err := rows.Scan(
			&provider.ID, &provider.Name, &provider.ProviderType, &provider.APIKey,
			&provider.APIEndpoint, &provider.Model, &provider.MaxTokens, &provider.Temperature,
			&provider.Enabled, &provider.CreatedAt, &provider.UpdatedAt, &provider.CreatedBy,
			&provider.TestStatus, &provider.TestMessage, &provider.TestAt,
		)
		if err != nil {
			return nil, err
		}
		providers = append(providers, provider)
	}

	return providers, nil
}

// UpdateAIProvider updates an existing AI provider
func (d *SQLDatabase) UpdateAIProvider(provider *AIProvider) error {
	provider.UpdatedAt = time.Now()

	query := `UPDATE ai_providers SET 
		name = ?, provider_type = ?, api_key = ?, api_endpoint = ?, model = ?,
		max_tokens = ?, temperature = ?, enabled = ?, updated_at = ?,
		test_status = ?, test_message = ?, test_at = ?
		WHERE id = ?`

	_, err := d.db.Exec(query,
		provider.Name, provider.ProviderType, provider.APIKey, provider.APIEndpoint,
		provider.Model, provider.MaxTokens, provider.Temperature, provider.Enabled,
		provider.UpdatedAt, provider.TestStatus, provider.TestMessage, provider.TestAt,
		provider.ID,
	)

	return err
}

// DeleteAIProvider deletes an AI provider
func (d *SQLDatabase) DeleteAIProvider(id string) error {
	query := `DELETE FROM ai_providers WHERE id = ?`
	_, err := d.db.Exec(query, id)
	return err
}

// CreateAIListener creates a new AI listener configuration
func (d *SQLDatabase) CreateAIListener(listener *AIListener) error {
	if listener.ID == "" {
		listener.ID = uuid.New().String()
	}

	now := time.Now()
	listener.CreatedAt = now
	listener.UpdatedAt = now

	query := `INSERT INTO ai_listeners (
		id, listener_id, ai_provider_id, openapi_spec, system_prompt,
		conversation_thread, generated_responses, last_generated_at,
		generation_status, generation_error, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := d.db.Exec(query,
		listener.ID, listener.ListenerID, listener.AIProviderID, listener.OpenAPISpec,
		listener.SystemPrompt, listener.ConversationThread, listener.GeneratedResponses,
		listener.LastGeneratedAt, listener.GenerationStatus, listener.GenerationError,
		listener.CreatedAt, listener.UpdatedAt,
	)

	return err
}

// GetAIListener retrieves an AI listener by ID
func (d *SQLDatabase) GetAIListener(id string) (*AIListener, error) {
	query := `SELECT id, listener_id, ai_provider_id, openapi_spec, system_prompt,
		conversation_thread, generated_responses, last_generated_at,
		generation_status, generation_error, created_at, updated_at
		FROM ai_listeners WHERE id = ?`

	listener := &AIListener{}
	err := d.db.QueryRow(query, id).Scan(
		&listener.ID, &listener.ListenerID, &listener.AIProviderID, &listener.OpenAPISpec,
		&listener.SystemPrompt, &listener.ConversationThread, &listener.GeneratedResponses,
		&listener.LastGeneratedAt, &listener.GenerationStatus, &listener.GenerationError,
		&listener.CreatedAt, &listener.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return listener, nil
}

// GetAIListenerByListenerID retrieves an AI listener by listener ID
func (d *SQLDatabase) GetAIListenerByListenerID(listenerID string) (*AIListener, error) {
	query := `SELECT id, listener_id, ai_provider_id, openapi_spec, system_prompt,
		conversation_thread, generated_responses, last_generated_at,
		generation_status, generation_error, created_at, updated_at
		FROM ai_listeners WHERE listener_id = ?`

	listener := &AIListener{}
	err := d.db.QueryRow(query, listenerID).Scan(
		&listener.ID, &listener.ListenerID, &listener.AIProviderID, &listener.OpenAPISpec,
		&listener.SystemPrompt, &listener.ConversationThread, &listener.GeneratedResponses,
		&listener.LastGeneratedAt, &listener.GenerationStatus, &listener.GenerationError,
		&listener.CreatedAt, &listener.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil // Not an AI listener
	}
	if err != nil {
		return nil, err
	}

	return listener, nil
}

// UpdateAIListener updates an existing AI listener
func (d *SQLDatabase) UpdateAIListener(listener *AIListener) error {
	listener.UpdatedAt = time.Now()

	query := `UPDATE ai_listeners SET 
		ai_provider_id = ?, openapi_spec = ?, system_prompt = ?,
		conversation_thread = ?, generated_responses = ?, last_generated_at = ?,
		generation_status = ?, generation_error = ?, updated_at = ?
		WHERE id = ?`

	_, err := d.db.Exec(query,
		listener.AIProviderID, listener.OpenAPISpec, listener.SystemPrompt,
		listener.ConversationThread, listener.GeneratedResponses, listener.LastGeneratedAt,
		listener.GenerationStatus, listener.GenerationError, listener.UpdatedAt,
		listener.ID,
	)

	return err
}

// DeleteAIListener deletes an AI listener
func (d *SQLDatabase) DeleteAIListener(id string) error {
	query := `DELETE FROM ai_listeners WHERE id = ?`
	_, err := d.db.Exec(query, id)
	return err
}

// AI Response Version methods

// CreateAIResponseVersion creates a new AI response version
func (d *SQLDatabase) CreateAIResponseVersion(version *AIResponseVersion) error {
	if version.ID == "" {
		version.ID = uuid.New().String()
	}

	query := `INSERT INTO ai_response_versions
		(id, ai_listener_id, version_number, openapi_spec, system_prompt, user_instructions,
		 generated_responses, generation_status, generation_error, is_active, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`

	_, err := d.db.Exec(query, version.ID, version.AIListenerID, version.VersionNumber,
		version.OpenAPISpec, version.SystemPrompt, version.UserInstructions,
		version.GeneratedResponses, version.GenerationStatus, version.GenerationError,
		version.IsActive)

	return err
}

// GetAIResponseVersion gets an AI response version by ID
func (d *SQLDatabase) GetAIResponseVersion(id string) (*AIResponseVersion, error) {
	var version AIResponseVersion
	query := `SELECT id, ai_listener_id, version_number, openapi_spec, system_prompt,
		user_instructions, generated_responses, generation_status, generation_error,
		is_active, created_at FROM ai_response_versions WHERE id = ?`

	row := d.db.QueryRow(query, id)
	err := row.Scan(&version.ID, &version.AIListenerID, &version.VersionNumber,
		&version.OpenAPISpec, &version.SystemPrompt, &version.UserInstructions,
		&version.GeneratedResponses, &version.GenerationStatus, &version.GenerationError,
		&version.IsActive, &version.CreatedAt)

	if err != nil {
		return nil, err
	}

	return &version, nil
}

// GetActiveAIResponseVersion gets the active AI response version for a listener
func (d *SQLDatabase) GetActiveAIResponseVersion(aiListenerID string) (*AIResponseVersion, error) {
	var version AIResponseVersion
	query := `SELECT id, ai_listener_id, version_number, openapi_spec, system_prompt,
		user_instructions, generated_responses, generation_status, generation_error,
		is_active, created_at FROM ai_response_versions
		WHERE ai_listener_id = ? AND is_active = 1 ORDER BY version_number DESC LIMIT 1`

	row := d.db.QueryRow(query, aiListenerID)
	err := row.Scan(&version.ID, &version.AIListenerID, &version.VersionNumber,
		&version.OpenAPISpec, &version.SystemPrompt, &version.UserInstructions,
		&version.GeneratedResponses, &version.GenerationStatus, &version.GenerationError,
		&version.IsActive, &version.CreatedAt)

	if err != nil {
		return nil, err
	}

	return &version, nil
}

// ListAIResponseVersions lists all versions for an AI listener
func (d *SQLDatabase) ListAIResponseVersions(aiListenerID string) ([]*AIResponseVersion, error) {
	var versions []*AIResponseVersion
	query := `SELECT id, ai_listener_id, version_number, openapi_spec, system_prompt,
		user_instructions, generated_responses, generation_status, generation_error,
		is_active, created_at FROM ai_response_versions
		WHERE ai_listener_id = ? ORDER BY version_number DESC`

	rows, err := d.db.Query(query, aiListenerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var version AIResponseVersion
		err := rows.Scan(&version.ID, &version.AIListenerID, &version.VersionNumber,
			&version.OpenAPISpec, &version.SystemPrompt, &version.UserInstructions,
			&version.GeneratedResponses, &version.GenerationStatus, &version.GenerationError,
			&version.IsActive, &version.CreatedAt)
		if err != nil {
			return nil, err
		}
		versions = append(versions, &version)
	}

	return versions, nil
}

// UpdateAIResponseVersion updates an AI response version
func (d *SQLDatabase) UpdateAIResponseVersion(version *AIResponseVersion) error {
	query := `UPDATE ai_response_versions SET
		openapi_spec = ?, system_prompt = ?, user_instructions = ?,
		generated_responses = ?, generation_status = ?, generation_error = ?,
		is_active = ? WHERE id = ?`

	_, err := d.db.Exec(query, version.OpenAPISpec, version.SystemPrompt,
		version.UserInstructions, version.GeneratedResponses, version.GenerationStatus,
		version.GenerationError, version.IsActive, version.ID)

	return err
}

// SetActiveAIResponseVersion sets a version as active and deactivates others
func (d *SQLDatabase) SetActiveAIResponseVersion(aiListenerID, versionID string) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Deactivate all versions for this AI listener
	_, err = tx.Exec("UPDATE ai_response_versions SET is_active = 0 WHERE ai_listener_id = ?", aiListenerID)
	if err != nil {
		return err
	}

	// Activate the specified version
	_, err = tx.Exec("UPDATE ai_response_versions SET is_active = 1 WHERE id = ?", versionID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// DeleteAIResponseVersion deletes an AI response version
func (d *SQLDatabase) DeleteAIResponseVersion(id string) error {
	_, err := d.db.Exec("DELETE FROM ai_response_versions WHERE id = ?", id)
	return err
}
