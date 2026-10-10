package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type taskRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	DueDate     string `json:"dueDate"`
	ContactID   string `json:"contactId"`
	Owner       string `json:"owner"`
}

type taskStatusRequest struct {
	Status string `json:"status"`
}

var validTaskStatuses = []string{"todo", "in_progress", "done"}

func isValidTaskStatus(s string) bool {
	for _, v := range validTaskStatuses {
		if v == s {
			return true
		}
	}
	return false
}

func (state *runtimeStore) validateTask(req taskRequest) map[string]string {
	fields := map[string]string{}
	title := strings.TrimSpace(req.Title)
	status := strings.TrimSpace(req.Status)
	contactID := strings.TrimSpace(req.ContactID)
	if title == "" {
		fields["title"] = "Title is required"
	} else if len(title) > 200 {
		fields["title"] = "Title must be 200 characters or fewer"
	}
	if status != "" && !isValidTaskStatus(status) {
		fields["status"] = "Invalid status"
	}
	if contactID != "" {
		if _, ok := state.findContactByID(contactID); !ok {
			fields["contactId"] = "Contact not found"
		}
	}
	return fields
}

func validateTask(req taskRequest) map[string]string {
	state, release := runtimeForReadCall(nil)
	defer release()
	return state.validateTask(req)
}

func (state *runtimeStore) listTasks(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	limit, offset := parseLimitOffset(r, 50, 0)
	state.mu.RLock()
	defer state.mu.RUnlock()
	filtered := make([]Task, 0, len(state.tasks))
	for _, t := range state.tasks {
		if statusFilter != "" && t.Status != statusFilter {
			continue
		}
		if q == "" ||
			strings.Contains(strings.ToLower(t.Title), q) ||
			strings.Contains(strings.ToLower(t.Description), q) ||
			strings.Contains(strings.ToLower(t.ContactName), q) ||
			strings.Contains(strings.ToLower(t.Owner), q) {
			filtered = append(filtered, t)
		}
	}
	filtered = sortRecords(filtered, r)
	total := len(filtered)
	writeJSON(w, http.StatusOK, map[string]any{
		"tasks": slicePage(filtered, offset, limit), "total": total, "limit": limit, "offset": offset,
	})
}

func listTasks(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForReadCall(r)
	defer release()
	state.listTasks(w, r)
}

func (state *runtimeStore) getTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	state.mu.RLock()
	defer state.mu.RUnlock()
	for _, t := range state.tasks {
		if t.ID == id {
			writeJSON(w, http.StatusOK, map[string]any{"task": t})
			return
		}
	}
	writeError(w, http.StatusNotFound, "Task not found")
}

func getTask(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForReadCall(r)
	defer release()
	state.getTask(w, r)
}

func (state *runtimeStore) createTask(w http.ResponseWriter, r *http.Request) {
	var req taskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if fields := state.validateTask(req); len(fields) > 0 {
		writeFieldErrors(w, fields)
		return
	}
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = "todo"
	}
	contactID := strings.TrimSpace(req.ContactID)
	contactName := ""
	if contactID != "" {
		if c, ok := state.findContactByID(contactID); ok {
			contactName = c.Name
		}
	}
	now := time.Now().Format("2006-01-02")
	newTask := Task{
		RecordScope: newRecordScope(r),
		ID:          newID("t"), Title: strings.TrimSpace(req.Title), Description: req.Description,
		Status: status, DueDate: strings.TrimSpace(req.DueDate),
		ContactID: contactID, ContactName: contactName,
		Owner: strings.TrimSpace(req.Owner), CreatedAt: now,
	}
	state.tasks = append(state.tasks, newTask)
	writeJSON(w, http.StatusCreated, map[string]any{"task": newTask})
}

func createTask(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.createTask(w, r)
}

func (state *runtimeStore) updateTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	state.mu.Lock()
	defer state.mu.Unlock()
	idx := -1
	for i, t := range state.tasks {
		if t.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		writeError(w, http.StatusNotFound, "Task not found")
		return
	}
	var req taskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if fields := state.validateTask(req); len(fields) > 0 {
		writeFieldErrors(w, fields)
		return
	}
	contactID := strings.TrimSpace(req.ContactID)
	contactName := ""
	if contactID != "" {
		if c, ok := state.findContactByID(contactID); ok {
			contactName = c.Name
		}
	}
	updated := state.tasks[idx]
	updated.Title = strings.TrimSpace(req.Title)
	updated.Description = req.Description
	updated.DueDate = strings.TrimSpace(req.DueDate)
	updated.ContactID = contactID
	updated.ContactName = contactName
	updated.Owner = strings.TrimSpace(req.Owner)
	if status := strings.TrimSpace(req.Status); status != "" {
		updated.Status = status
	}
	state.tasks[idx] = updated
	writeJSON(w, http.StatusOK, map[string]any{"task": updated})
}

func updateTask(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.updateTask(w, r)
}

func (state *runtimeStore) updateTaskStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	state.mu.Lock()
	defer state.mu.Unlock()
	idx := -1
	for i, t := range state.tasks {
		if t.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		writeError(w, http.StatusNotFound, "Task not found")
		return
	}
	var req taskStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	status := strings.TrimSpace(req.Status)
	if !isValidTaskStatus(status) {
		writeFieldErrors(w, map[string]string{"status": "Invalid status"})
		return
	}
	state.tasks[idx].Status = status
	writeJSON(w, http.StatusOK, map[string]any{"task": state.tasks[idx]})
}

func updateTaskStatus(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.updateTaskStatus(w, r)
}

func (state *runtimeStore) deleteTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	state.mu.Lock()
	defer state.mu.Unlock()
	idx := -1
	for i, t := range state.tasks {
		if t.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		writeError(w, http.StatusNotFound, "Task not found")
		return
	}
	state.tasks = append(state.tasks[:idx], state.tasks[idx+1:]...)
	writeJSON(w, http.StatusOK, map[string]any{"deleted": id})
}

func deleteTask(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.deleteTask(w, r)
}
