package tasks

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

var (
	ErrTaskNotFound = errors.New("task not found")
)

// TaskRunner defines the function signature for a background task worker.
type TaskRunner func(ctx context.Context, t *Task) error

// Manager coordinates in-memory background tasks.
type Manager struct {
	tasks map[string]*Task
	order []string
	mu    sync.RWMutex
}

var (
	defaultManager     *Manager
	defaultManagerOnce sync.Once
)

// DefaultManager returns the singleton background task manager instance.
func DefaultManager() *Manager {
	defaultManagerOnce.Do(func() {
		defaultManager = NewManager()
	})
	return defaultManager
}

// NewManager creates a new TaskManager.
func NewManager() *Manager {
	return &Manager{
		tasks: make(map[string]*Task),
		order: make([]string, 0),
	}
}

func generateTaskID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("task-%d-%s", time.Now().Unix(), hex.EncodeToString(b))
}

// Start launches a new background task in a detached goroutine.
func (m *Manager) Start(taskType TaskType, title, target string, runner TaskRunner) *Task {
	id := generateTaskID()
	now := time.Now()

	ctx, cancel := context.WithCancel(context.Background())

	task := &Task{
		ID:           id,
		Type:         taskType,
		Title:        title,
		Target:       target,
		Status:       StatusRunning,
		Progress:     0,
		CurrentStage: "Initializing...",
		Logs:         make([]string, 0),
		CreatedAt:    now,
		StartedAt:    &now,
		cancel:       cancel,
	}

	m.mu.Lock()
	m.tasks[id] = task
	m.order = append(m.order, id)
	// Prune old completed/failed tasks if over 100
	const maxStoredTasks = 100
	if len(m.order) > maxStoredTasks {
		toRemove := m.order[0 : len(m.order)-maxStoredTasks]
		m.order = m.order[len(m.order)-maxStoredTasks:]
		for _, remID := range toRemove {
			if t, exists := m.tasks[remID]; exists && t.Status != StatusRunning && t.Status != StatusPending {
				delete(m.tasks, remID)
			}
		}
	}
	m.mu.Unlock()

	// Execute worker in detached goroutine
	go func() {
		defer func() {
			if r := recover(); r != nil {
				task.Fail(fmt.Errorf("panic in task worker: %v", r))
			}
		}()

		err := runner(ctx, task)
		if err != nil {
			if errors.Is(ctx.Err(), context.Canceled) {
				task.mu.Lock()
				task.Status = StatusCancelled
				finished := time.Now()
				task.FinishedAt = &finished
				task.mu.Unlock()
			} else {
				task.Fail(err)
			}
		} else {
			task.Complete()
		}
	}()

	return task
}

// Get returns the snapshot of a specific task by ID.
func (m *Manager) Get(id string) (*Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.tasks[id]
	if !exists {
		return nil, ErrTaskNotFound
	}

	snap := task.Snapshot()
	return &snap, nil
}

// List returns snapshots of all tasks, sorted newest first.
func (m *Manager) List() []Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Task, 0, len(m.tasks))
	for _, t := range m.tasks {
		result = append(result, t.Snapshot())
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result
}

// Cancel requests cancellation of a running task by ID.
func (m *Manager) Cancel(id string) error {
	m.mu.RLock()
	task, exists := m.tasks[id]
	m.mu.RUnlock()

	if !exists {
		return ErrTaskNotFound
	}

	task.Cancel()
	return nil
}

// Delete removes a finished task from memory.
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[id]
	if !exists {
		return ErrTaskNotFound
	}

	if task.GetStatus() == StatusRunning {
		return errors.New("cannot delete an active running task")
	}

	delete(m.tasks, id)
	newOrder := make([]string, 0, len(m.order))
	for _, item := range m.order {
		if item != id {
			newOrder = append(newOrder, item)
		}
	}
	m.order = newOrder

	return nil
}
