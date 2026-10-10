package main

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"
)

type activityRequest struct {
	Kind       string `json:"kind"`
	Title      string `json:"title"`
	Body       string `json:"body"`
	OccurredAt string `json:"occurredAt"`
	DealID     string `json:"dealId"`
}

var validActivityKinds = []string{"email", "whatsapp", "call", "meeting", "note", "task"}

func isValidActivityKind(k string) bool {
	for _, v := range validActivityKinds {
		if v == k {
			return true
		}
	}
	return false
}

// validateActivity must be called with mu held.
func validateActivity(req activityRequest) map[string]string {
	fields := map[string]string{}
	kind := strings.TrimSpace(req.Kind)
	title := strings.TrimSpace(req.Title)

	if kind == "" {
		fields["kind"] = "Kind is required"
	} else if !isValidActivityKind(kind) {
		fields["kind"] = "Invalid kind"
	}

	if title == "" {
		fields["title"] = "Title is required"
	} else if len(title) > 200 {
		fields["title"] = "Title must be 200 characters or fewer"
	}

	return fields
}

// normalizeDate returns an ISO-8601 date (YYYY-MM-DD).
func normalizeDate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Now().Format("2006-01-02")
	}
	if len(s) > 10 {
		return s[:10]
	}
	return s
}

func (state *runtimeStore) listActivities(w http.ResponseWriter, r *http.Request) {
	contactID := r.PathValue("id")

	state.mu.RLock()
	defer state.mu.RUnlock()

	found := false
	for _, c := range state.contacts {
		if c.ID == contactID {
			found = true
			break
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, "Contact not found")
		return
	}

	filtered := make([]Activity, 0)
	for _, a := range state.activities {
		if a.ContactID == contactID {
			filtered = append(filtered, a)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].OccurredAt > filtered[j].OccurredAt
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"activities": filtered,
		"total":      len(filtered),
	})
}

func listActivities(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForReadCall(r)
	defer release()
	state.listActivities(w, r)
}

func (state *runtimeStore) createActivity(w http.ResponseWriter, r *http.Request) {
	contactID := r.PathValue("id")

	var req activityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	contactIdx := -1
	for i, c := range state.contacts {
		if c.ID == contactID {
			contactIdx = i
			break
		}
	}
	if contactIdx == -1 {
		writeError(w, http.StatusNotFound, "Contact not found")
		return
	}

	if fields := validateActivity(req); len(fields) > 0 {
		writeFieldErrors(w, fields)
		return
	}

	dealID := strings.TrimSpace(req.DealID)
	if dealID != "" {
		ok := false
		for _, d := range state.deals {
			if d.ID == dealID {
				ok = true
				break
			}
		}
		if !ok {
			writeFieldErrors(w, map[string]string{"dealId": "Deal not found"})
			return
		}
	}

	occurredAt := normalizeDate(req.OccurredAt)
	now := time.Now().Format("2006-01-02")

	newActivity := Activity{
		RecordScope: newRecordScope(r),
		ID:          newID("a"),
		ContactID:   contactID,
		DealID:      dealID,
		Kind:        strings.TrimSpace(req.Kind),
		Title:       strings.TrimSpace(req.Title),
		Body:        req.Body,
		OccurredAt:  occurredAt,
		CreatedBy:   "Demo User",
		CreatedAt:   now,
	}
	state.activities = append(state.activities, newActivity)
	for i := range state.contacts {
		if state.contacts[i].ID == contactID && state.contacts[i].LastContact < occurredAt {
			state.contacts[i].LastContact = occurredAt
		}
	}

	if occurredAt > state.contacts[contactIdx].LastContact {
		state.contacts[contactIdx].LastContact = occurredAt
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"activity": newActivity,
		"contact":  state.contacts[contactIdx],
	})
}

func createActivity(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.createActivity(w, r)
}

func (state *runtimeStore) deleteActivity(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	state.mu.Lock()
	defer state.mu.Unlock()

	idx := -1
	for i, a := range state.activities {
		if a.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		writeError(w, http.StatusNotFound, "Activity not found")
		return
	}

	state.activities = append(state.activities[:idx], state.activities[idx+1:]...)
	writeJSON(w, http.StatusOK, map[string]any{"deleted": id})
}

func deleteActivity(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.deleteActivity(w, r)
}
