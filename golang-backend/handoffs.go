package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type ApplicationHandoff struct {
	ID         string `json:"id"`
	CaseID     string `json:"caseId"`
	FromID     string `json:"fromId"`
	ToID       string `json:"toId"`
	FromName   string `json:"fromName"`
	ToName     string `json:"toName"`
	Reason     string `json:"reason"`
	NextAction string `json:"nextAction"`
	DueDate    string `json:"dueDate"`
	Revision   string `json:"revision"`
	Status     string `json:"status"`
	CreatedAt  string `json:"createdAt"`
	ResolvedAt string `json:"resolvedAt,omitempty"`
}

func registerHandoffRoutes(m *http.ServeMux) {
	m.HandleFunc("GET /api/cases/{id}/handoffs", authMiddleware(handleApplicationHandoffs))
	m.HandleFunc("POST /api/cases/{id}/handoffs", authMiddleware(handleProposeHandoff))
	m.HandleFunc("POST /api/handoffs/{handoff}/{action}", authMiddleware(handleResolveHandoff))
}
func handoffHistory(r *http.Request, caseID string) ([]ApplicationHandoff, error) {
	rows, err := storeDB(r).Query("SELECT id,data FROM workspace_entries WHERE org_id=? AND category='application_handoff' AND record_id=? ORDER BY created_at DESC,id DESC", currentUser(r).OrgID, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ApplicationHandoff{}
	for rows.Next() {
		var h ApplicationHandoff
		var raw string
		if err = rows.Scan(&h.ID, &raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(raw), &h); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}
func handleApplicationHandoffs(w http.ResponseWriter, r *http.Request) {
	state, done := runtimeForCall(r)
	defer done()
	c, ok := state.findCaseByID(r.PathValue("id"))
	if !ok {
		writeError(w, 404, "Application not found")
		return
	}
	if owner, found := state.findUserByID(c.OwnerID); found && owner.OrgID == c.OrgID {
		c.AssignedOfficer = owner.Name
	}
	history, err := handoffHistory(r, c.ID)
	if err != nil {
		writeError(w, 500, "Could not load handoffs")
		return
	}
	recipients := []User{}
	for _, u := range state.users {
		if u.OrgID == c.OrgID && !u.Disabled && u.Role != "viewer" && u.ID != c.OwnerID && canRead(c.RecordScope, u) {
			u.Password = ""
			recipients = append(recipients, u)
		}
	}
	writeJSON(w, 200, map[string]any{"today": organizationToday(r), "ownerId": c.OwnerID, "owner": c.AssignedOfficer, "nextAction": c.NextAction, "dueDate": c.NextDeadline, "history": history, "recipients": recipients, "canPropose": canWrite(c.RecordScope, currentUser(r)), "userId": currentUser(r).ID})
}
func handleProposeHandoff(w http.ResponseWriter, r *http.Request) {
	state, done := runtimeForCall(r)
	defer done()
	c, ok := state.findCaseByID(r.PathValue("id"))
	if !ok {
		writeError(w, 404, "Application not found")
		return
	}
	if !canWrite(c.RecordScope, currentUser(r)) {
		writeError(w, 403, "Only the owner or a workspace manager can propose a handoff")
		return
	}
	var h ApplicationHandoff
	if json.NewDecoder(r.Body).Decode(&h) != nil {
		writeError(w, 400, "Invalid handoff")
		return
	}
	recipient, ok := state.findUserByID(h.ToID)
	if !ok || recipient.OrgID != c.OrgID || recipient.Disabled || recipient.Role == "viewer" || recipient.ID == c.OwnerID || !canRead(c.RecordScope, recipient) {
		writeError(w, 400, "Choose an active staff member who can access this application")
		return
	}
	h.Reason = strings.TrimSpace(h.Reason)
	h.NextAction = strings.TrimSpace(h.NextAction)
	if h.Reason == "" || h.NextAction == "" || len(h.Reason) > 2000 || len(h.NextAction) > 500 {
		writeError(w, 400, "Enter a reason and next action within the field limits")
		return
	}
	date, err := time.Parse("2006-01-02", h.DueDate)
	if err != nil || date.Format("2006-01-02") < organizationToday(r) {
		writeError(w, 400, "Choose today or a future due date")
		return
	}
	history, err := handoffHistory(r, c.ID)
	if err != nil {
		writeError(w, 500, "Could not check pending handoffs")
		return
	}
	for _, old := range history {
		if old.Status == "pending" {
			writeError(w, 409, "Resolve or cancel the pending handoff first")
			return
		}
	}
	var record map[string]any
	json.Unmarshal([]byte(mustJSON(c)), &record)
	h.ID = newID("handoff")
	h.CaseID = c.ID
	h.FromID = c.OwnerID
	h.FromName = c.AssignedOfficer
	if owner, found := state.findUserByID(c.OwnerID); found && owner.OrgID == c.OrgID {
		h.FromName = owner.Name
	}
	h.ToName = recipient.Name
	h.Status = "pending"
	h.CreatedAt = utcNow()
	h.ResolvedAt = ""
	h.Revision = versionOf(record)
	if _, err = storeDB(r).Exec("INSERT INTO workspace_entries VALUES(?,?, 'application_handoff',?,?,?,?,?)", h.ID, c.OrgID, currentUser(r).ID, c.ID, mustJSON(h), h.CreatedAt, h.CreatedAt); err != nil {
		writeError(w, 500, "Could not save handoff")
		return
	}
	_, err = storeDB(r).Exec("INSERT OR IGNORE INTO notifications VALUES(?,?,?,?,?,?,?,?,?,NULL)", newID("notice"), c.OrgID, recipient.ID, "handoff", "Application handoff requested", h.NextAction, "/cases/"+c.ID, "handoff:"+h.ID, utcNow())
	if err != nil {
		writeError(w, 500, "Could not notify receiving staff")
		return
	}
	writeJSON(w, 201, map[string]any{"handoff": h})
}
func handleResolveHandoff(w http.ResponseWriter, r *http.Request) {
	state, done := runtimeForCall(r)
	defer done()
	var raw string
	if storeDB(r).QueryRow("SELECT data FROM workspace_entries WHERE id=? AND org_id=? AND category='application_handoff'", r.PathValue("handoff"), currentUser(r).OrgID).Scan(&raw) != nil {
		writeError(w, 404, "Handoff not found")
		return
	}
	var h ApplicationHandoff
	if json.Unmarshal([]byte(raw), &h) != nil {
		writeError(w, 500, "Could not read handoff")
		return
	}
	c, ok := state.findCaseByID(h.CaseID)
	if !ok {
		writeError(w, 404, "Application not found")
		return
	}
	if h.Status != "pending" {
		writeError(w, 409, "This handoff has already been resolved")
		return
	}
	action := r.PathValue("action")
	u := currentUser(r)
	if action == "cancel" {
		if !canWrite(c.RecordScope, u) {
			writeError(w, 403, "Only the current owner or a manager can cancel this handoff")
			return
		}
		h.Status = "cancelled"
	} else if action == "accept" || action == "decline" {
		if u.ID != h.ToID || u.Disabled || u.Role == "viewer" {
			writeError(w, 403, "Only the receiving staff member can accept or decline")
			return
		}
		if action == "decline" {
			h.Status = "declined"
		} else {
			var record map[string]any
			json.Unmarshal([]byte(mustJSON(c)), &record)
			if c.OwnerID != h.FromID || versionOf(record) != h.Revision {
				writeError(w, 409, "The application changed. Ask its owner to cancel and propose a fresh handoff")
				return
			}
			meta := metadata(r)
			if meta == nil {
				writeError(w, 500, "Transaction metadata unavailable")
				return
			}
			meta.OwnershipTransfers = map[string]bool{"cases:" + c.ID: true}
			for i := range state.cases {
				if state.cases[i].ID == c.ID {
					state.cases[i].OwnerID = u.ID
					state.cases[i].AssignedOfficer = u.Name
					state.cases[i].NextAction = h.NextAction
					state.cases[i].NextDeadline = h.DueDate
				}
			}
			rows, err := storeDB(r).Query("SELECT data FROM documents WHERE json_extract(data,'$.orgId')=? AND json_extract(data,'$.caseId')=? AND json_extract(data,'$.ownerId')=?", u.OrgID, c.ID, h.FromID)
			if err != nil {
				writeError(w, 500, "Could not transfer document ownership")
				return
			}
			owned := []Document{}
			for rows.Next() {
				var raw string
				var d Document
				if err = rows.Scan(&raw); err != nil {
					break
				}
				if err = json.Unmarshal([]byte(raw), &d); err != nil {
					break
				}
				owned = append(owned, d)
			}
			if rows.Err() != nil {
				err = rows.Err()
			}
			rows.Close()
			if err != nil {
				writeError(w, 500, "Could not transfer document ownership")
				return
			}
			for _, d := range owned {
				meta.OwnershipTransfers["documents:"+d.ID] = true
				d.OwnerID = u.ID
				found := false
				for i := range state.documents {
					if state.documents[i].ID == d.ID {
						state.documents[i] = d
						found = true
						break
					}
				}
				if !found {
					state.documents = append(state.documents, d)
				}
			}

			h.Status = "accepted"
		}
	} else {
		writeError(w, 400, "Choose accept, decline or cancel")
		return
	}
	h.ResolvedAt = utcNow()
	result, err := storeDB(r).Exec("UPDATE workspace_entries SET data=?,updated_at=? WHERE id=? AND org_id=? AND data=?", mustJSON(h), h.ResolvedAt, h.ID, u.OrgID, raw)
	if err != nil {
		writeError(w, 500, "Could not resolve handoff")
		return
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		writeError(w, 409, "Handoff changed; refresh and try again")
		return
	}
	if _, err = storeDB(r).Exec("UPDATE notifications SET read_at=COALESCE(read_at,?) WHERE org_id=? AND (dedupe_key=? OR instr(dedupe_key,?)=1)", utcNow(), c.OrgID, "handoff:"+h.ID, "handoff-reminder:"+h.ID+":"); err != nil {
		writeError(w, 500, "Could not resolve handoff notifications")
		return
	}
	writeJSON(w, 200, map[string]any{"handoff": h})
}
