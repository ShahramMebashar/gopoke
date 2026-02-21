package env

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"sync"
)

// Variable describes one project environment variable.
type Variable struct {
	Key    string
	Value  string
	Masked bool
}

// Service manages per-project environment variables in memory.
type Service struct {
	mu     sync.RWMutex
	values map[string]map[string]Variable
}

// NewService creates an environment variable service.
func NewService() *Service {
	return &Service{
		values: make(map[string]map[string]Variable),
	}
}

// Upsert sets a variable for a project.
func (s *Service) Upsert(ctx context.Context, projectID string, variable Variable) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("upsert env context: %w", err)
	}
	if projectID == "" {
		return fmt.Errorf("project ID is required")
	}
	if variable.Key == "" {
		return fmt.Errorf("variable key is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.values[projectID]; !ok {
		s.values[projectID] = make(map[string]Variable)
	}
	s.values[projectID][variable.Key] = variable
	return nil
}

// Delete removes a variable for a project.
func (s *Service) Delete(ctx context.Context, projectID string, key string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("delete env context: %w", err)
	}
	if projectID == "" {
		return fmt.Errorf("project ID is required")
	}
	if key == "" {
		return fmt.Errorf("variable key is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	projectVars, ok := s.values[projectID]
	if !ok {
		return nil
	}
	delete(projectVars, key)
	return nil
}

// List returns all variables for a project sorted by key.
func (s *Service) List(ctx context.Context, projectID string) ([]Variable, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("list env context: %w", err)
	}
	if projectID == "" {
		return nil, fmt.Errorf("project ID is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	projectVars, ok := s.values[projectID]
	if !ok {
		return nil, nil
	}

	cloned := maps.Clone(projectVars)
	items := make([]Variable, 0, len(cloned))
	for _, variable := range cloned {
		items = append(items, variable)
	}

	slices.SortFunc(items, func(a, b Variable) int {
		switch {
		case a.Key < b.Key:
			return -1
		case a.Key > b.Key:
			return 1
		default:
			return 0
		}
	})

	return items, nil
}
