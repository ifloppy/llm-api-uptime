package store

import (
	"database/sql"
	"fmt"
	"llm-api-uptime/internal/model"
	"time"
)

func (s *Store) CreateProbe(p *model.Probe) error {
	now := time.Now()
	result, err := s.db.Exec(
		"INSERT INTO probes (provider_id, model, enabled, created_at) VALUES (?, ?, ?, ?)",
		p.ProviderID, p.Model, p.Enabled, now,
	)
	if err != nil {
		return fmt.Errorf("insert probe: %w", err)
	}
	id, _ := result.LastInsertId()
	p.ID = id
	p.CreatedAt = now
	return nil
}

func (s *Store) GetProbe(id int64) (*model.Probe, error) {
	p := &model.Probe{}
	err := s.db.QueryRow(
		"SELECT id, provider_id, model, enabled, created_at FROM probes WHERE id = ?",
		id,
	).Scan(&p.ID, &p.ProviderID, &p.Model, &p.Enabled, &p.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("probe not found: %d", id)
	}
	if err != nil {
		return nil, fmt.Errorf("get probe: %w", err)
	}
	return p, nil
}

func (s *Store) ListProbes(providerID int64) ([]model.Probe, error) {
	rows, err := s.db.Query("SELECT id, provider_id, model, enabled, created_at FROM probes WHERE provider_id = ? ORDER BY model", providerID)
	if err != nil {
		return nil, fmt.Errorf("list probes: %w", err)
	}
	defer rows.Close()

	var probes []model.Probe
	for rows.Next() {
		var p model.Probe
		if err := rows.Scan(&p.ID, &p.ProviderID, &p.Model, &p.Enabled, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan probe: %w", err)
		}
		probes = append(probes, p)
	}
	return probes, nil
}

func (s *Store) ListAllProbes() ([]model.Probe, error) {
	rows, err := s.db.Query("SELECT id, provider_id, model, enabled, created_at FROM probes ORDER BY provider_id, model")
	if err != nil {
		return nil, fmt.Errorf("list all probes: %w", err)
	}
	defer rows.Close()

	var probes []model.Probe
	for rows.Next() {
		var p model.Probe
		if err := rows.Scan(&p.ID, &p.ProviderID, &p.Model, &p.Enabled, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan probe: %w", err)
		}
		probes = append(probes, p)
	}
	return probes, nil
}

func (s *Store) GetEnabledProbes() ([]model.ProbeWithProvider, error) {
	rows, err := s.db.Query(`
		SELECT p.id, p.provider_id, p.model, p.enabled, p.created_at,
		       pr.name, pr.base_url, pr.api_key, pr.api_type
		FROM probes p
		JOIN providers pr ON p.provider_id = pr.id
		WHERE p.enabled = 1 AND pr.enabled = 1
		ORDER BY pr.name, p.model
	`)
	if err != nil {
		return nil, fmt.Errorf("list enabled probes: %w", err)
	}
	defer rows.Close()

	var probes []model.ProbeWithProvider
	for rows.Next() {
		var p model.ProbeWithProvider
		if err := rows.Scan(&p.ID, &p.ProviderID, &p.Model, &p.Enabled, &p.CreatedAt,
			&p.ProviderName, &p.ProviderURL, &p.APIKey, &p.APIType); err != nil {
			return nil, fmt.Errorf("scan probe: %w", err)
		}
		probes = append(probes, p)
	}
	return probes, nil
}

func (s *Store) UpdateProbe(p *model.Probe) error {
	_, err := s.db.Exec(
		"UPDATE probes SET model = ?, enabled = ? WHERE id = ?",
		p.Model, p.Enabled, p.ID,
	)
	if err != nil {
		return fmt.Errorf("update probe: %w", err)
	}
	return nil
}

func (s *Store) DeleteProbe(id int64) error {
	_, err := s.db.Exec("DELETE FROM probes WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete probe: %w", err)
	}
	return nil
}

func (s *Store) DeleteProbesByProvider(providerID int64) error {
	_, err := s.db.Exec("DELETE FROM probes WHERE provider_id = ?", providerID)
	if err != nil {
		return fmt.Errorf("delete probes by provider: %w", err)
	}
	return nil
}
