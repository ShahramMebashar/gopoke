package snippet

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

// Record stores a snippet and its metadata.
type Record struct {
	ID        string
	ProjectID string
	Name      string
	Content   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Service provides in-memory snippet CRUD for early development.
type Service struct {
	mu      sync.RWMutex
	counter atomic.Uint64
	records map[string]Record
}

// NewService creates a snippet service.
func NewService() *Service {
	return &Service{
		records: make(map[string]Record),
	}
}

// Save inserts or updates a snippet.
func (s *Service) Save(ctx context.Context, record Record) (Record, error) {
	if err := ctx.Err(); err != nil {
		return Record{}, fmt.Errorf("save snippet context: %w", err)
	}
	if record.ProjectID == "" {
		return Record{}, fmt.Errorf("project ID is required")
	}
	if record.Name == "" {
		return Record{}, fmt.Errorf("name is required")
	}

	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	if record.ID == "" {
		record.ID = fmt.Sprintf("sn_%d", s.counter.Add(1))
		record.CreatedAt = now
	}
	record.UpdatedAt = now
	s.records[record.ID] = record
	return record, nil
}

// Get returns a snippet by ID.
func (s *Service) Get(ctx context.Context, id string) (Record, error) {
	if err := ctx.Err(); err != nil {
		return Record{}, fmt.Errorf("get snippet context: %w", err)
	}
	if id == "" {
		return Record{}, fmt.Errorf("snippet ID is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	record, ok := s.records[id]
	if !ok {
		return Record{}, fmt.Errorf("snippet not found")
	}
	return record, nil
}

// Delete removes a snippet by ID.
func (s *Service) Delete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("delete snippet context: %w", err)
	}
	if id == "" {
		return fmt.Errorf("snippet ID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.records, id)
	return nil
}

// ListByProject returns snippets for a project, ordered by newest update first.
func (s *Service) ListByProject(ctx context.Context, projectID string) ([]Record, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("list snippets context: %w", err)
	}
	if projectID == "" {
		return nil, fmt.Errorf("project ID is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	records := make([]Record, 0, len(s.records))
	for _, record := range s.records {
		if record.ProjectID == projectID {
			records = append(records, record)
		}
	}

	slices.SortFunc(records, func(a, b Record) int {
		if a.UpdatedAt.After(b.UpdatedAt) {
			return -1
		}
		if a.UpdatedAt.Before(b.UpdatedAt) {
			return 1
		}
		return 0
	})

	return records, nil
}
