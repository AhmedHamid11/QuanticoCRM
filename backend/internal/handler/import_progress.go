package handler

import (
	"sync"
	"time"
)

// ImportJob tracks the progress of a background import operation.
type ImportJob struct {
	ID        string              `json:"id"`
	Status    string              `json:"status"` // "running", "complete", "error"
	Progress  ImportProgressEvent `json:"progress"`
	Response  *ImportCSVResponse  `json:"response,omitempty"`
	Error     string              `json:"error,omitempty"`
	CreatedAt time.Time           `json:"-"`
}

// ImportProgressStore is an in-memory store for active import jobs.
type ImportProgressStore struct {
	mu   sync.RWMutex
	jobs map[string]*ImportJob
}

// Global singleton — import jobs are ephemeral and don't need persistence.
var importProgressStore = &ImportProgressStore{
	jobs: make(map[string]*ImportJob),
}

func init() {
	// Clean up stale jobs every 5 minutes
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			importProgressStore.cleanup()
		}
	}()
}

func (s *ImportProgressStore) Create(id string, total int) *ImportJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	job := &ImportJob{
		ID:     id,
		Status: "running",
		Progress: ImportProgressEvent{
			Type:  "progress",
			Total: total,
			Phase: "importing",
		},
		CreatedAt: time.Now(),
	}
	s.jobs[id] = job
	return job
}

func (s *ImportProgressStore) Get(id string) *ImportJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.jobs[id]
}

func (s *ImportProgressStore) Update(id string, event ImportProgressEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if job, ok := s.jobs[id]; ok {
		job.Progress = event
	}
}

func (s *ImportProgressStore) Complete(id string, response *ImportCSVResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if job, ok := s.jobs[id]; ok {
		job.Status = "complete"
		job.Response = response
		// Update progress to reflect final state
		job.Progress.Processed = job.Progress.Total
	}
}

func (s *ImportProgressStore) SetError(id string, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if job, ok := s.jobs[id]; ok {
		job.Status = "error"
		job.Error = errMsg
	}
}

// cleanup removes completed/errored jobs older than 10 minutes.
func (s *ImportProgressStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := time.Now().Add(-10 * time.Minute)
	for id, job := range s.jobs {
		if job.Status != "running" && job.CreatedAt.Before(cutoff) {
			delete(s.jobs, id)
		}
	}
}
