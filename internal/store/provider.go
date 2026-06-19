package store

import (
	"database/sql"
	"fmt"
	"llm-api-uptime/internal/model"
	"time"
)

func (s *Store) CreateProvider(p *model.Provider) error {
	if p.MaxTokens <= 0 {
		p.MaxTokens = 2
	}
	now := time.Now()
	result, err := s.db.Exec(
		"INSERT INTO providers (name, base_url, api_key, api_type, max_tokens, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		p.Name, p.BaseURL, p.APIKey, p.APIType, p.MaxTokens, p.Enabled, now, now,
	)
	if err != nil {
		return fmt.Errorf("insert provider: %w", err)
	}
	id, _ := result.LastInsertId()
	p.ID = id
	p.CreatedAt = now
	p.UpdatedAt = now
	return nil
}

func (s *Store) GetProvider(id int64) (*model.Provider, error) {
	p := &model.Provider{}
	err := s.db.QueryRow(
		"SELECT id, name, base_url, api_key, api_type, max_tokens, enabled, created_at, updated_at FROM providers WHERE id = ?",
		id,
	).Scan(&p.ID, &p.Name, &p.BaseURL, &p.APIKey, &p.APIType, &p.MaxTokens, &p.Enabled, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("provider not found: %d", id)
	}
	if err != nil {
		return nil, fmt.Errorf("get provider: %w", err)
	}
	return p, nil
}

func (s *Store) ListProviders() ([]model.Provider, error) {
	rows, err := s.db.Query("SELECT id, name, base_url, api_key, api_type, max_tokens, enabled, created_at, updated_at FROM providers ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("list providers: %w", err)
	}
	defer rows.Close()

	var providers []model.Provider
	for rows.Next() {
		var p model.Provider
		if err := rows.Scan(&p.ID, &p.Name, &p.BaseURL, &p.APIKey, &p.APIType, &p.MaxTokens, &p.Enabled, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan provider: %w", err)
		}
		providers = append(providers, p)
	}
	return providers, nil
}

func (s *Store) UpdateProvider(p *model.Provider) error {
	if p.MaxTokens <= 0 {
		p.MaxTokens = 2
	}
	now := time.Now()
	_, err := s.db.Exec(
		"UPDATE providers SET name = ?, base_url = ?, api_key = ?, api_type = ?, max_tokens = ?, enabled = ?, updated_at = ? WHERE id = ?",
		p.Name, p.BaseURL, p.APIKey, p.APIType, p.MaxTokens, p.Enabled, now, p.ID,
	)
	if err != nil {
		return fmt.Errorf("update provider: %w", err)
	}
	p.UpdatedAt = now
	return nil
}

func (s *Store) DeleteProvider(id int64) error {
	_, err := s.db.Exec("DELETE FROM providers WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete provider: %w", err)
	}
	return nil
}

func (s *Store) GetEnabledProviders() ([]model.Provider, error) {
	rows, err := s.db.Query("SELECT id, name, base_url, api_key, api_type, max_tokens, enabled, created_at, updated_at FROM providers WHERE enabled = 1 ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("list enabled providers: %w", err)
	}
	defer rows.Close()

	var providers []model.Provider
	for rows.Next() {
		var p model.Provider
		if err := rows.Scan(&p.ID, &p.Name, &p.BaseURL, &p.APIKey, &p.APIType, &p.MaxTokens, &p.Enabled, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan provider: %w", err)
		}
		providers = append(providers, p)
	}
	return providers, nil
}
