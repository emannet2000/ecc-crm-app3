package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func rawUserRequest(t *testing.T, h http.Handler, u User, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	token, err := issueToken(u)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(method, path, strings.NewReader(mustJSON(body)))
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func TestApplicationHandoffAcceptancePermissionsAndHistory(t *testing.T) {
	setupStore(t)
	owner, _ := findUserByEmail("demo@northwind.dev")
	recipient := owner
	recipient.ID = "handoff-recipient"
	recipient.Email = "recipient@example.com"
	recipient.Name = "Receiving Adviser"
	recipient.Role = "member"
	viewer := recipient
	viewer.ID = "handoff-viewer"
	viewer.Email = "viewer@example.com"
	viewer.Role = "viewer"
	mu.Lock()
	users[recipient.Email] = recipient
	users[viewer.Email] = viewer
	cases[0].OwnerID = owner.ID
	cases[0].AssignedOfficer = owner.Name
	documents[0].OwnerID = owner.ID
	documents[0].Visibility = "private"
	mu.Unlock()
	if err := saveStore(); err != nil {
		t.Fatal(err)
	}
	h := expansionTestMux()
	caseID := cases[0].ID
	path := "/api/cases/" + caseID + "/handoffs"
	w := rawUserRequest(t, h, owner, "POST", path, map[string]any{"toId": recipient.ID, "reason": "Continue document review", "nextAction": "Check passport evidence", "dueDate": "2099-01-01"})
	assertStatus(t, w, 201)
	var out struct{ Handoff ApplicationHandoff }
	json.Unmarshal(w.Body.Bytes(), &out)
	var record string
	database.QueryRow("SELECT data FROM cases WHERE id=?", caseID).Scan(&record)
	var c Case
	json.Unmarshal([]byte(record), &c)
	if c.OwnerID != owner.ID {
		t.Fatal("proposal changed ownership before acceptance")
	}
	assertStatus(t, rawUserRequest(t, h, owner, "POST", path, map[string]any{"toId": recipient.ID, "reason": "duplicate", "nextAction": "Check", "dueDate": "2099-01-01"}), 409)
	endpoint := "/api/handoffs/" + out.Handoff.ID + "/accept"
	assertStatus(t, rawUserRequest(t, h, owner, "POST", endpoint, map[string]any{}), 403)
	assertStatus(t, rawUserRequest(t, h, viewer, "POST", endpoint, map[string]any{}), 403)
	queue := rawUserRequest(t, h, recipient, "GET", "/api/my-work", nil)
	assertStatus(t, queue, 200)
	if !strings.Contains(queue.Body.String(), out.Handoff.ID) {
		t.Fatal("receiving staff has no handoff in daily work")
	}
	assertStatus(t, rawUserRequest(t, h, recipient, "POST", endpoint, map[string]any{}), 200)
	database.QueryRow("SELECT data FROM cases WHERE id=?", caseID).Scan(&record)
	json.Unmarshal([]byte(record), &c)
	if c.OwnerID != recipient.ID || c.AssignedOfficer != recipient.Name || c.NextAction != "Check passport evidence" {
		t.Fatal("handoff did not transfer ownership and next action")
	}
	var doc Document
	database.QueryRow("SELECT data FROM documents WHERE id=?", documents[0].ID).Scan(&record)
	json.Unmarshal([]byte(record), &doc)
	if doc.OwnerID != recipient.ID {
		t.Fatal("owned private document was not transferred")
	}
	assertStatus(t, rawUserRequest(t, h, recipient, "POST", endpoint, map[string]any{}), 409)
	history := rawUserRequest(t, h, recipient, "GET", path, nil)
	assertStatus(t, history, 200)
	if !strings.Contains(history.Body.String(), "accepted") {
		t.Fatal("handoff history missing")
	}
	// The live unit of work must not publish edits into the bootstrap cache.
	if cases[0].OwnerID != owner.ID {
		t.Fatal("runtime writes leaked into shared package state")
	}
}
func TestApplicationHandoffRejectsChangedCaseAndForeignRecipient(t *testing.T) {
	setupStore(t)
	owner, _ := findUserByEmail("demo@northwind.dev")
	recipient := owner
	recipient.ID = "handoff-new"
	recipient.Email = "handoff-new@example.com"
	recipient.Role = "member"
	foreign := recipient
	foreign.ID = "handoff-foreign"
	foreign.Email = "foreign-handoff@example.com"
	foreign.OrgID = "another-workspace"
	mu.Lock()
	users[recipient.Email] = recipient
	users[foreign.Email] = foreign
	mu.Unlock()
	if err := saveStore(); err != nil {
		t.Fatal(err)
	}
	h := expansionTestMux()
	path := "/api/cases/" + cases[0].ID + "/handoffs"
	assertStatus(t, rawUserRequest(t, h, owner, "POST", path, map[string]any{"toId": foreign.ID, "reason": "test", "nextAction": "test", "dueDate": "2099-01-01"}), 400)
	w := rawUserRequest(t, h, owner, "POST", path, map[string]any{"toId": recipient.ID, "reason": "Review", "nextAction": "Continue", "dueDate": "2099-01-01"})
	assertStatus(t, w, 201)
	var out struct{ Handoff ApplicationHandoff }
	json.Unmarshal(w.Body.Bytes(), &out)
	if _, err := database.Exec("UPDATE cases SET data=json_set(data,'$.nextAction','Newer instruction') WHERE id=?", cases[0].ID); err != nil {
		t.Fatal(err)
	}
	assertStatus(t, rawUserRequest(t, h, recipient, "POST", "/api/handoffs/"+out.Handoff.ID+"/accept", nil), 409)
	assertStatus(t, rawUserRequest(t, h, owner, "POST", "/api/handoffs/"+out.Handoff.ID+"/cancel", nil), 200)
}
