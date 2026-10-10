package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

type UsabilityObservation struct {
	Role       string `json:"role"`
	Task       string `json:"task"`
	Completed  bool   `json:"completed"`
	Seconds    int    `json:"seconds"`
	Errors     int    `json:"errors"`
	NeededHelp bool   `json:"neededHelp"`
	Comments   string `json:"comments"`
}

func handleUsabilityFeedback(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if r.Method == "GET" {
		where := "org_id=? AND category='usability_feedback'"
		args := []any{u.OrgID}
		if !isManager(u) {
			where += " AND user_id=?"
			args = append(args, u.ID)
		}
		rows, err := queryObjects(r, "SELECT id,user_id AS userId,data,created_at AS createdAt FROM workspace_entries WHERE "+where+" ORDER BY created_at DESC LIMIT 200", args...)
		if err != nil {
			writeError(w, 500, "Could not load observations")
			return
		}
		writeJSON(w, 200, map[string]any{"observations": rows, "canReviewAll": isManager(u)})
		return
	}
	var in UsabilityObservation
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		writeError(w, 400, "Invalid observation")
		return
	}
	validTasks := map[string]string{"daily-priority": "adviser", "student-review": "adviser", "application-handoff": "adviser", "invoice-receipt": "finance", "recovery-mobile": "either"}
	role, ok := validTasks[in.Task]
	if !ok || (in.Role != "adviser" && in.Role != "finance") || (role != "either" && role != in.Role) || in.Seconds < 0 || in.Seconds > 86400 || in.Errors < 0 || in.Errors > 1000 {
		writeError(w, 400, "Choose a matching task and role, with valid time and error counts")
		return
	}
	in.Comments = strings.TrimSpace(in.Comments)
	if in.Comments == "" || len(in.Comments) > 4000 {
		writeError(w, 400, "Enter observations within 4,000 characters")
		return
	}
	id, err := entry(r, "usability_feedback", in.Task, in, true)
	if err != nil {
		writeError(w, 500, "Could not save observation")
		return
	}
	writeJSON(w, 201, map[string]any{"id": id, "observation": in})
}
