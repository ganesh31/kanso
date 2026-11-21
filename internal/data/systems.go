package data

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"kanso/internal/data/db"
	"kanso/internal/market"
	"kanso/internal/rules"
)

// SaveSystem stores sys as JSON and returns its id. A system without an id
// gets a new UUID; one with an id replaces the stored system with that id
// (or is created under it).
func (s *Store) SaveSystem(ctx context.Context, sys rules.System) (string, error) {
	if err := s.writable(); err != nil {
		return "", err
	}
	if sys.ID == "" {
		sys.ID = uuid.NewString()
	}
	def, err := json.Marshal(sys)
	if err != nil {
		return "", fmt.Errorf("encode system: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	err = s.q.UpsertSystem(ctx, db.UpsertSystemParams{
		ID: sys.ID, Name: sys.Name, Definition: string(def), CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return "", fmt.Errorf("save system %s: %w", sys.ID, err)
	}
	return sys.ID, nil
}

// GetSystem returns the stored system, or an error wrapping
// market.ErrNotFound.
func (s *Store) GetSystem(ctx context.Context, id string) (rules.System, error) {
	def, err := s.q.GetSystem(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return rules.System{}, fmt.Errorf("system %q: %w", id, market.ErrNotFound)
	}
	if err != nil {
		return rules.System{}, fmt.Errorf("system %q: %w", id, err)
	}
	return decodeSystem(def)
}

// ListSystems returns every stored system, sorted by name.
func (s *Store) ListSystems(ctx context.Context) ([]rules.System, error) {
	defs, err := s.q.ListSystems(ctx)
	if err != nil {
		return nil, fmt.Errorf("list systems: %w", err)
	}
	out := make([]rules.System, 0, len(defs))
	for _, def := range defs {
		sys, err := decodeSystem(def)
		if err != nil {
			return nil, err
		}
		out = append(out, sys)
	}
	return out, nil
}

// DeleteSystem removes a stored system; an unknown id is an error wrapping
// market.ErrNotFound.
func (s *Store) DeleteSystem(ctx context.Context, id string) error {
	if err := s.writable(); err != nil {
		return err
	}
	n, err := s.q.DeleteSystem(ctx, id)
	if err != nil {
		return fmt.Errorf("delete system %q: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("system %q: %w", id, market.ErrNotFound)
	}
	return nil
}

func decodeSystem(def string) (rules.System, error) {
	var sys rules.System
	if err := json.Unmarshal([]byte(def), &sys); err != nil {
		return rules.System{}, fmt.Errorf("decode system: %w", err)
	}
	return sys, nil
}
