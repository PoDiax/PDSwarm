package tasks

import (
	"context"
	"sync"
	"time"
)

// TaskStatus represents the lifecycle state of a background task.
type TaskStatus string

const (
	StatusPending   TaskStatus = "pending"
	StatusRunning   TaskStatus = "running"
	StatusCompleted TaskStatus = "completed"
	StatusFailed    TaskStatus = "failed"
	StatusCancelled TaskStatus = "cancelled"
)

// TaskType represents the category of the background operation.
type TaskType string

const (
	TypeImagePull         TaskType = "image_pull"
	TypeImageBuild        TaskType = "image_build"
	TypeContainerUpdate   TaskType = "container_update"
	TypeContainerRecreate TaskType = "container_recreate"
	TypeStackDeploy       TaskType = "stack_deploy"
	TypeStackDown         TaskType = "stack_down"
	TypeTemplateDeploy    TaskType = "template_deploy"
	TypePrune             TaskType = "prune"
	TypeGeneral           TaskType = "general"
)

// Task represents a background job running independently of browser connections.
type Task struct {
	ID           string     `json:"id"`
	Type         TaskType   `json:"type"`
	Title        string     `json:"title"`
	Target       string     `json:"target,omitempty"`
	Status       TaskStatus `json:"status"`
	Progress     int        `json:"progress"` // 0 - 100
	CurrentStage string     `json:"current_stage,omitempty"`
	Logs         []string   `json:"logs"`
	Error        string     `json:"error,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`

	cancel context.CancelFunc
	mu     sync.RWMutex
}

// Snapshot returns a shallow copy of the Task for thread-safe JSON serialization.
func (t *Task) Snapshot() Task {
	t.mu.RLock()
	defer t.mu.RUnlock()

	logsCopy := make([]string, len(t.Logs))
	copy(logsCopy, t.Logs)

	return Task{
		ID:           t.ID,
		Type:         t.Type,
		Title:        t.Title,
		Target:       t.Target,
		Status:       t.Status,
		Progress:     t.Progress,
		CurrentStage: t.CurrentStage,
		Logs:         logsCopy,
		Error:        t.Error,
		CreatedAt:    t.CreatedAt,
		StartedAt:    t.StartedAt,
		FinishedAt:   t.FinishedAt,
	}
}

// SetProgress updates the progress percentage and current stage description.
func (t *Task) SetProgress(pct int, stage string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if pct < 0 {
		pct = 0
	} else if pct > 100 {
		pct = 100
	}
	t.Progress = pct
	if stage != "" {
		t.CurrentStage = stage
	}
}

// AppendLog adds a log line to the task's in-memory ring buffer (up to 1000 lines).
func (t *Task) AppendLog(line string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	const maxLogs = 1000
	if len(t.Logs) >= maxLogs {
		t.Logs = t.Logs[len(t.Logs)-maxLogs+1:]
	}
	t.Logs = append(t.Logs, line)
}

// Complete marks the task as completed successfully.
func (t *Task) Complete() {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	t.Status = StatusCompleted
	t.Progress = 100
	t.FinishedAt = &now
}

// Fail marks the task as failed with an error message.
func (t *Task) Fail(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	t.Status = StatusFailed
	if err != nil {
		t.Error = err.Error()
	}
	t.FinishedAt = &now
}

// Cancel terminates the task via its context.CancelFunc.
func (t *Task) Cancel() {
	t.mu.Lock()
	cancel := t.cancel
	t.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	t.Status = StatusCancelled
	t.FinishedAt = &now
	const maxLogs = 1000
	if len(t.Logs) >= maxLogs {
		t.Logs = t.Logs[len(t.Logs)-maxLogs+1:]
	}
	t.Logs = append(t.Logs, "Task cancelled by user.")
}
