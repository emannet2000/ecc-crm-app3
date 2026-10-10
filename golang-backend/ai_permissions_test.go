package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func addPermissionUser(t *testing.T, role, org, id string) User {
	t.Helper()
	u := User{ID: id, Name: id, Email: id + "@example.com", Role: role, OrgID: org}
	mu.Lock()
	users[u.Email] = u
	mu.Unlock()
	if err := saveStore(); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec("INSERT OR IGNORE INTO organizations(id,name,timezone,currency,created_at) VALUES(?, ?, 'UTC', 'USD', ?)", org, org, utcNow()); err != nil {
		t.Fatal(err)
	}
	return u
}
func enablePermissionAI(t *testing.T, h http.Handler) {
	t.Helper()
	assertStatus(t, expansionRequest(t, h, "PUT", "/api/admin/assistant", map[string]any{"enabled": true, "actions": true}, ""), 200)
}
func TestAIConfirmationOwnershipStalenessAndRoleRevocation(t *testing.T) {
	setupStore(t)
	h := expansionTestMux()
	admin, _ := findUserByEmail("demo@northwind.dev")
	enablePermissionAI(t, h)
	member := addPermissionUser(t, "member", admin.OrgID, "ai-owner")
	other := addPermissionUser(t, "member", admin.OrgID, "ai-other")
	foreign := addPermissionUser(t, "admin", "foreign-ai-org", "ai-foreign")
	w := dailyUserRequest(t, h, member, "POST", "/api/contacts", map[string]any{"name": "Before proposal", "email": "proposal@example.com", "stage": "Lead"})
	assertStatus(t, w, 201)
	var created struct{ Contact Contact }
	json.Unmarshal(w.Body.Bytes(), &created)
	proposal := func() aiAction {
		t.Helper()
		a, err := prepareAIAction(aiUserRequest(context.Background(), member), "", "update_record", "contacts", created.Contact.ID, map[string]any{"name": "After confirmation"})
		if err != nil {
			t.Fatal(err)
		}
		return a
	}
	action := proposal()
	path := "/api/assistant/actions/" + action.ID + "/confirm"
	assertStatus(t, dailyUserRequest(t, h, other, "POST", path, nil), 404)
	assertStatus(t, dailyUserRequest(t, h, foreign, "POST", path, nil), 404)
	read := dailyUserRequest(t, h, member, "GET", "/api/contacts/"+created.Contact.ID, nil)
	if json.Unmarshal(read.Body.Bytes(), &created) != nil || created.Contact.Name != "Before proposal" {
		t.Fatal("proposal changed the record before confirmation")
	}
	assertStatus(t, dailyUserRequest(t, h, member, "POST", path, nil), 200)
	assertStatus(t, dailyUserRequest(t, h, member, "POST", path, nil), 200)
	action = proposal()
	assertStatus(t, dailyUserRequest(t, h, member, "PUT", "/api/contacts/"+created.Contact.ID, map[string]any{"name": "Intervening edit", "email": "proposal@example.com", "stage": "Lead"}), 200)
	assertStatus(t, dailyUserRequest(t, h, member, "POST", "/api/assistant/actions/"+action.ID+"/confirm", nil), 409)
	action = proposal()
	member.Role = "viewer"
	mu.Lock()
	users[member.Email] = member
	mu.Unlock()
	if err := saveStore(); err != nil {
		t.Fatal(err)
	}
	assertStatus(t, dailyUserRequest(t, h, member, "POST", "/api/assistant/actions/"+action.ID+"/confirm", nil), 403)
	if _, err := prepareAIAction(aiUserRequest(context.Background(), member), "", "create_task", "", "", map[string]any{"title": "Forbidden"}); err == nil {
		t.Fatal("viewer could propose an action")
	}
}
func TestKnowledgeVisibilityOwnershipAndRevisionGuards(t *testing.T) {
	setupStore(t)
	h := expansionTestMux()
	admin, _ := findUserByEmail("demo@northwind.dev")
	owner := addPermissionUser(t, "member", admin.OrgID, "knowledge-owner")
	other := addPermissionUser(t, "member", admin.OrgID, "knowledge-other")
	viewer := addPermissionUser(t, "viewer", admin.OrgID, "knowledge-viewer")
	foreign := addPermissionUser(t, "admin", "knowledge-other-org", "knowledge-foreign")
	body := map[string]any{"title": "Application procedure", "content": "Review the passport", "category": "procedure", "visibility": "organization"}
	w := dailyUserRequest(t, h, owner, "POST", "/api/assistant/knowledge", body)
	assertStatus(t, w, 201)
	var source aiKnowledge
	json.Unmarshal(w.Body.Bytes(), &source)
	if source.ID == "" {
		t.Fatal(w.Body.String())
	}
	path := "/api/assistant/knowledge/" + source.ID
	assertStatus(t, dailyUserRequest(t, h, viewer, "GET", path, nil), 200)
	assertStatus(t, dailyUserRequest(t, h, foreign, "GET", path, nil), 404)
	assertStatus(t, dailyUserRequest(t, h, viewer, "POST", "/api/assistant/knowledge", body), 403)
	assertStatus(t, dailyUserRequest(t, h, other, "PUT", path, body), 403)
	assertStatus(t, dailyUserRequest(t, h, owner, "PUT", path, body), 409)
	token, _ := issueToken(owner)
	req := permissionRequest("PUT", path, body, token, source.Revision)
	response := servePermissionRequest(h, req)
	assertStatus(t, response, 201)
	assertStatus(t, servePermissionRequest(h, permissionRequest("PUT", path, body, token, "stale-version")), 409)
	assertStatus(t, dailyUserRequest(t, h, foreign, "DELETE", path, nil), 404)
}

func permissionRequest(method, path string, body any, token, version string) *http.Request {
	r := httptest.NewRequest(method, path, strings.NewReader(mustJSON(body)))
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("If-Match", version)
	return r
}
func servePermissionRequest(h http.Handler, r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	serveFixtureRequest(h, w, r)
	return w
}
