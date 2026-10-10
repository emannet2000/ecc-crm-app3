package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func dailyUserRequest(t *testing.T, h http.Handler, u User, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	token, err := issueToken(u)
	if err != nil {
		t.Fatal(err)
	}
	payload := ""
	if body != nil {
		payload = mustJSON(body)
	}
	r := httptest.NewRequest(method, path, strings.NewReader(payload))
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	serveFixtureRequest(h, w, r)
	return w
}
func TestMyWorkFullCountsScopeAndPagination(t *testing.T) {
	setupStore(t)
	u, _ := findUserByEmail("demo@northwind.dev")
	scope := RecordScope{OrgID: u.OrgID, OwnerID: u.ID, Visibility: "organization"}
	mu.Lock()
	cases = nil
	tasks = nil
	leads = nil
	documents = nil
	for i := 0; i < 60; i++ {
		cases = append(cases, Case{RecordScope: scope, ID: fmt.Sprintf("daily_%03d", i), CaseNumber: fmt.Sprintf("DAILY-%03d", i), CurrentStage: "Documents Pending", AssignedOfficer: u.Name, NextAction: "Review passport", NextDeadline: "2099-01-01"})
	}
	hidden := cases[0]
	hidden.ID = "other-org"
	hidden.OrgID = "other-org"
	cases = append(cases, hidden)
	other := cases[0]
	other.ID = "other-owner"
	other.AssignedOfficer = "Another adviser"
	cases = append(cases, other)
	documents = append(documents, Document{RecordScope: scope, ID: "required", CaseID: "daily_000", DocName: "Passport", Required: true, Status: "Requested"})
	mu.Unlock()
	if err := saveStore(); err != nil {
		t.Fatal(err)
	}
	h := expansionTestMux()
	w := dailyUserRequest(t, h, u, "GET", "/api/my-work?scope=mine&limit=25&offset=25", nil)
	assertStatus(t, w, 200)
	var out struct {
		Items  []productivityItem
		Total  int
		Counts map[string]int
	}
	json.Unmarshal(w.Body.Bytes(), &out)
	if out.Total != 60 || len(out.Items) != 25 || out.Counts["applications"] != 60 || out.Counts["documents"] != 1 {
		t.Fatalf("wrong totals: %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "other-org") || strings.Contains(w.Body.String(), "other-owner") {
		t.Fatal("assignment or workspace leaked")
	}
	w = dailyUserRequest(t, h, u, "GET", "/api/my-work?scope=all", nil)
	assertStatus(t, w, 200)
	json.Unmarshal(w.Body.Bytes(), &out)
	if out.Total != 61 {
		t.Fatal(w.Body.String())
	}
	assertStatus(t, dailyUserRequest(t, h, u, "GET", "/api/my-work?scope=invalid", nil), 400)
}
func TestJourneyKeepsStudentCaseAndMessageBoundaries(t *testing.T) {
	setupStore(t)
	u, _ := findUserByEmail("demo@northwind.dev")
	scope := RecordScope{OrgID: u.OrgID, OwnerID: u.ID, Visibility: "organization"}
	mu.Lock()
	students = []Student{{RecordScope: scope, ID: "journey-student", Name: "Student"}}
	cases = []Case{{RecordScope: scope, ID: "journey-case", StudentID: "journey-student", CaseNumber: "J-1"}, {RecordScope: scope, ID: "unrelated-case", StudentID: "another-student"}}
	documents = []Document{{RecordScope: scope, ID: "journey-doc", CaseID: "journey-case", DocName: "Passport", Required: true, Status: "Verified"}, {RecordScope: scope, ID: "unrelated-doc", CaseID: "unrelated-case", DocName: "Secret"}}
	invoices = []Invoice{{RecordScope: scope, ID: "journey-invoice", CaseID: "journey-case", Currency: "PHP", Balance: 50}}
	payments = []Payment{{RecordScope: scope, ID: "journey-payment", InvoiceID: "journey-invoice", Amount: 20, Currency: "PHP"}}
	mu.Unlock()
	if err := saveStore(); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"journey-case", "unrelated-case", ""} {
		if _, err := database.Exec("INSERT INTO conversations VALUES(?,?,?,?,?,?,?,?,?,?,?)", "message-"+id, u.OrgID, "shared-contact", id, "email", "inbound", "provider-"+id, "Subject", "Body "+id, "received", utcNow()); err != nil {
			t.Fatal(err)
		}
	}
	w := dailyUserRequest(t, expansionTestMux(), u, "GET", "/api/students/journey-student/journey", nil)
	assertStatus(t, w, 200)
	var out struct {
		Applications       []Case
		Documents          []Document
		Invoices           []Invoice
		Payments           []Payment
		Messages           []map[string]any
		Required, Verified int
	}
	json.Unmarshal(w.Body.Bytes(), &out)
	if len(out.Applications) != 1 || len(out.Documents) != 1 || len(out.Invoices) != 1 || len(out.Payments) != 1 || len(out.Messages) != 1 || out.Verified != 1 || out.Required != 1 {
		t.Fatal(w.Body.String())
	}
	if strings.Contains(w.Body.String(), "unrelated") || strings.Contains(w.Body.String(), "Secret") {
		t.Fatal("unrelated student data leaked")
	}
	assertStatus(t, dailyUserRequest(t, expansionTestMux(), u, "GET", "/api/students/absent/journey", nil), 404)
}
func TestInlineQualityUsesVisibleRecordsAndEditExclusion(t *testing.T) {
	setupStore(t)
	u, _ := findUserByEmail("demo@northwind.dev")
	u.ID = "quality-member"
	u.Name = "Quality member"
	u.Email = "quality-member@example.com"
	u.Role = "member"
	mu.Lock()
	users[u.Email] = u
	visible := RecordScope{OrgID: u.OrgID, OwnerID: u.ID, Visibility: "private"}
	hidden := visible
	hidden.OwnerID = "other-user"
	students = []Student{{RecordScope: visible, ID: "visible-student", Name: "Visible", StudentCode: "DUP"}, {RecordScope: hidden, ID: "hidden-student", Name: "Private secret", StudentCode: "DUP"}}
	mu.Unlock()
	if err := saveStore(); err != nil {
		t.Fatal(err)
	}
	h := expansionTestMux()
	w := dailyUserRequest(t, h, u, "POST", "/api/form-quality", map[string]any{"entity": "students", "studentCode": "DUP"})
	assertStatus(t, w, 200)
	if !strings.Contains(w.Body.String(), "Visible") || strings.Contains(w.Body.String(), "Private secret") {
		t.Fatal(w.Body.String())
	}
	w = dailyUserRequest(t, h, u, "POST", "/api/form-quality", map[string]any{"entity": "students", "id": "visible-student", "studentCode": "DUP"})
	assertStatus(t, w, 200)
	if strings.Contains(w.Body.String(), "duplicate") {
		t.Fatal("current edit matched itself")
	}
	assertStatus(t, dailyUserRequest(t, h, u, "POST", "/api/form-quality", map[string]any{"entity": "students", "id": "hidden-student"}), 404)
	w = dailyUserRequest(t, h, u, "POST", "/api/form-quality", map[string]any{"entity": "leads", "status": "New"})
	assertStatus(t, w, 200)
	if !strings.Contains(w.Body.String(), "follow-up date") {
		t.Fatal(w.Body.String())
	}
}
func TestIntegrationHealthChecksPersistFailuresAndInvalidateChangedSettings(t *testing.T) {
	setupStore(t)
	t.Setenv("STRIPE_SECRET_KEY", "fixture-key")
	t.Setenv("SMTP_HOST", "")
	t.Setenv("OPENAI_API_KEY", "")
	u, _ := findUserByEmail("demo@northwind.dev")
	h := expansionTestMux()
	previous := integrationProbeHTTP
	t.Cleanup(func() { integrationProbeHTTP = previous })
	calls := 0
	integrationProbeHTTP = &http.Client{Transport: testTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		key, _, _ := r.BasicAuth()
		if key != "fixture-key" || r.Method != "GET" || r.URL.Path != "/v1/account" {
			t.Fatal("unexpected provider request")
		}
		return &http.Response{StatusCode: 401, Body: io.NopCloser(strings.NewReader("secret-provider-response")), Header: make(http.Header)}, nil
	})}
	w := dailyUserRequest(t, h, u, "POST", "/api/admin/integrations/payments/test", nil)
	assertStatus(t, w, 200)
	if !strings.Contains(w.Body.String(), "failing") || strings.Contains(w.Body.String(), "fixture-key") || strings.Contains(w.Body.String(), "secret-provider") {
		t.Fatal(w.Body.String())
	}
	w = dailyUserRequest(t, h, u, "GET", "/api/integrations/status", nil)
	assertStatus(t, w, 200)
	if !strings.Contains(w.Body.String(), "failing") {
		t.Fatal(w.Body.String())
	}
	t.Setenv("STRIPE_SECRET_KEY", "changed-key")
	w = dailyUserRequest(t, h, u, "GET", "/api/integrations/status", nil)
	if strings.Contains(w.Body.String(), "failing") {
		t.Fatal("old test survived credential change")
	}
	viewer := u
	viewer.ID = "integration-viewer"
	viewer.Email = "integration-viewer@example.com"
	viewer.Role = "viewer"
	mu.Lock()
	users[viewer.Email] = viewer
	mu.Unlock()
	if err := saveStore(); err != nil {
		t.Fatal(err)
	}
	assertStatus(t, dailyUserRequest(t, h, viewer, "POST", "/api/admin/integrations/payments/test", nil), 403)
	if calls != 1 {
		t.Fatal("viewer reached provider")
	}
}
func TestRepositoryPagingLiteralSearchAndParentScope(t *testing.T) {
	setupStore(t)
	u, _ := findUserByEmail("demo@northwind.dev")
	u.ID = "repo-member"
	u.Email = "repo-member@example.com"
	u.Role = "member"
	scope := RecordScope{OrgID: u.OrgID, OwnerID: u.ID, Visibility: "private"}
	mu.Lock()
	users[u.Email] = u
	contacts = nil
	for i := 0; i < 55; i++ {
		contacts = append(contacts, Contact{RecordScope: scope, ID: fmt.Sprintf("repo-%02d", i), Name: fmt.Sprintf("Name %02d", i), Email: "literal%_@example.com", Stage: "Lead"})
	}
	hidden := scope
	hidden.OwnerID = "different-member"
	cases = []Case{{RecordScope: hidden, ID: "private-parent"}}
	documents = []Document{{RecordScope: scope, ID: "visible-doc-hidden-parent", CaseID: "private-parent", DocName: "Do not leak"}}
	mu.Unlock()
	if err := saveStore(); err != nil {
		t.Fatal(err)
	}
	h := expansionTestMux()
	w := dailyUserRequest(t, h, u, "GET", "/api/contacts?q=%25_&limit=10&offset=50&sort=name&direction=desc", nil)
	assertStatus(t, w, 200)
	var out struct {
		Contacts []Contact
		Total    int
	}
	json.Unmarshal(w.Body.Bytes(), &out)
	if out.Total != 55 || len(out.Contacts) != 5 || out.Contacts[0].Name != "Name 04" {
		t.Fatal(w.Body.String())
	}
	w = dailyUserRequest(t, h, u, "GET", "/api/records/documents/visible-doc-hidden-parent", nil)
	assertStatus(t, w, 404)
	m := http.NewServeMux()
	m.HandleFunc("GET /api/documents", authMiddleware(listDocumentsHandler))
	w = dailyUserRequest(t, persistMutations(m), u, "GET", "/api/documents", nil)
	assertStatus(t, w, 200)
	if strings.Contains(w.Body.String(), "Do not leak") {
		t.Fatal("hidden parent leaked through list")
	}
}
func TestDeltaWritesProtectConcurrentSQLChanges(t *testing.T) {
	setupStore(t)
	before := cloneStore()
	after := cloneStore()
	after.Contacts[0].Name = "Pending cache edit"
	id := after.Contacts[0].ID
	var payload string
	database.QueryRow("SELECT data FROM contacts WHERE id=?", id).Scan(&payload)
	var record map[string]any
	json.Unmarshal([]byte(payload), &record)
	record["name"] = "Committed SQL edit"
	database.Exec("UPDATE contacts SET data=? WHERE id=?", mustJSON(record), id)
	tx, err := database.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err = saveStoreDeltaTx(tx, before, after, nil); err == nil {
		t.Fatal("stale cache write overwrote a committed SQL change")
	}
}

func TestAnonymousLogoutCompletesWithDeltaPersistence(t *testing.T) {
	setupStore(t)
	m := http.NewServeMux()
	m.HandleFunc("POST /api/session/logout", handleSessionLogout)
	h := persistMutations(m)
	r := httptest.NewRequest("POST", "/api/session/logout", nil)
	w := httptest.NewRecorder()
	serveFixtureRequest(h, w, r)
	assertStatus(t, w, 200)
}
