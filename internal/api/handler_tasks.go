package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/podiax/pdswarm/internal/tasks"
)

// HandleListTasks returns all active and recent background tasks.
func HandleListTasks(mgr *tasks.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if mgr == nil {
			mgr = tasks.DefaultManager()
		}

		taskList := mgr.List()
		respondJSON(w, http.StatusOK, taskList)
	}
}

// HandleGetTask returns detail and logs for a specific background task.
func HandleGetTask(mgr *tasks.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if mgr == nil {
			mgr = tasks.DefaultManager()
		}

		id := chi.URLParam(r, "id")
		if strings.TrimSpace(id) == "" {
			respondJSONError(w, http.StatusBadRequest, "task id is required")
			return
		}

		task, err := mgr.Get(id)
		if err != nil {
			if errors.Is(err, tasks.ErrTaskNotFound) {
				respondJSONError(w, http.StatusNotFound, "task not found")
				return
			}
			respondJSONError(w, http.StatusInternalServerError, "failed to get task: "+err.Error())
			return
		}

		respondJSON(w, http.StatusOK, task)
	}
}

// HandleCancelTask cancels a running background task.
func HandleCancelTask(mgr *tasks.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if mgr == nil {
			mgr = tasks.DefaultManager()
		}

		id := chi.URLParam(r, "id")
		if strings.TrimSpace(id) == "" {
			respondJSONError(w, http.StatusBadRequest, "task id is required")
			return
		}

		if err := mgr.Cancel(id); err != nil {
			if errors.Is(err, tasks.ErrTaskNotFound) {
				respondJSONError(w, http.StatusNotFound, "task not found")
				return
			}
			respondJSONError(w, http.StatusInternalServerError, "failed to cancel task: "+err.Error())
			return
		}

		respondJSON(w, http.StatusOK, map[string]string{
			"message": "task cancelled",
			"id":      id,
		})
	}
}

// HandleDeleteTask removes a finished task from memory.
func HandleDeleteTask(mgr *tasks.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if mgr == nil {
			mgr = tasks.DefaultManager()
		}

		id := chi.URLParam(r, "id")
		if strings.TrimSpace(id) == "" {
			respondJSONError(w, http.StatusBadRequest, "task id is required")
			return
		}

		if err := mgr.Delete(id); err != nil {
			if errors.Is(err, tasks.ErrTaskNotFound) {
				respondJSONError(w, http.StatusNotFound, "task not found")
				return
			}
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}

		respondJSON(w, http.StatusOK, map[string]string{
			"message": "task deleted",
			"id":      id,
		})
	}
}
