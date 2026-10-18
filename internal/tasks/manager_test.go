package tasks_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/podiax/pdswarm/internal/tasks"
)

func TestTaskManager_Lifecycle(t *testing.T) {
	mgr := tasks.NewManager()

	task := mgr.Start(tasks.TypeImagePull, "Pulling test image", "nginx:alpine", func(ctx context.Context, task *tasks.Task) error {
		task.SetProgress(25, "Downloading layer 1")
		task.AppendLog("Step 1/2 complete")
		time.Sleep(10 * time.Millisecond)
		task.SetProgress(100, "Extracting")
		task.AppendLog("Step 2/2 complete")
		return nil
	})

	if task.ID == "" {
		t.Fatal("expected task ID to be non-empty")
	}

	// Poll until completed
	var finished *tasks.Task
	for i := 0; i < 50; i++ {
		snap, err := mgr.Get(task.ID)
		if err != nil {
			t.Fatalf("unexpected error getting task: %v", err)
		}
		if snap.Status == tasks.StatusCompleted {
			finished = snap
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if finished == nil {
		t.Fatal("task did not complete in time")
	}
	if finished.Progress != 100 {
		t.Fatalf("expected progress 100, got %d", finished.Progress)
	}
	if len(finished.Logs) < 2 {
		t.Fatalf("expected at least 2 logs, got %d", len(finished.Logs))
	}
}

func TestTaskManager_Cancel(t *testing.T) {
	mgr := tasks.NewManager()

	task := mgr.Start(tasks.TypeImagePull, "Long pull", "ubuntu:latest", func(ctx context.Context, task *tasks.Task) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
			return nil
		}
	})

	time.Sleep(10 * time.Millisecond)
	if err := mgr.Cancel(task.ID); err != nil {
		t.Fatalf("failed to cancel task: %v", err)
	}

	time.Sleep(20 * time.Millisecond)
	snap, err := mgr.Get(task.ID)
	if err != nil {
		t.Fatalf("failed to get task: %v", err)
	}
	if snap.Status != tasks.StatusCancelled {
		t.Fatalf("expected status cancelled, got %s", snap.Status)
	}
}

func TestTaskManager_Failure(t *testing.T) {
	mgr := tasks.NewManager()

	task := mgr.Start(tasks.TypeImagePull, "Failing pull", "unknown:invalid", func(ctx context.Context, task *tasks.Task) error {
		return errors.New("registry denied access")
	})

	time.Sleep(20 * time.Millisecond)
	snap, err := mgr.Get(task.ID)
	if err != nil {
		t.Fatalf("failed to get task: %v", err)
	}
	if snap.Status != tasks.StatusFailed {
		t.Fatalf("expected status failed, got %s", snap.Status)
	}
	if snap.Error != "registry denied access" {
		t.Fatalf("expected error message 'registry denied access', got '%s'", snap.Error)
	}
}
