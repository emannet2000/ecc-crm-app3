package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func expansionTestMux() http.Handler {
	m := http.NewServeMux()
	registerWorkspaceRoutes(m)
	registerFollowupRoutes(m)
	m.HandleFunc("GET /api/admin/operations", authMiddleware(handleOperations))
	m.HandleFunc("GET /api/contacts", authMiddleware(listContacts))
	m.HandleFunc("GET /api/deals", authMiddleware(listDeals))
	m.HandleFunc("GET /api/tasks", authMiddleware(listTasks))
	m.HandleFunc("GET /api/schools", authMiddleware(listSchoolsHandler))
	m.HandleFunc("GET /api/schools/{id}", authMiddleware(getSchoolHandler))
	m.HandleFunc("GET /api/students", authMiddleware(listStudentsHandler))
	m.HandleFunc("POST /api/students", authMiddleware(createStudentHandler))
	m.HandleFunc("GET /api/agents", authMiddleware(listAgentsHandler))
	m.HandleFunc("GET /api/agents/{id}", authMiddleware(getAgentHandler))
	m.HandleFunc("GET /api/leads", authMiddleware(listLeadsHandler))
	m.HandleFunc("GET /api/cases", authMiddleware(listCasesHandler))
	m.HandleFunc("GET /api/invoices", authMiddleware(listInvoicesHandler))
	m.HandleFunc("GET /api/partners", authMiddleware(listPartnersHandler))
	m.HandleFunc("GET /api/contacts/{id}", authMiddleware(getContact))
	m.HandleFunc("POST /api/contacts", authMiddleware(createContact))
	m.HandleFunc("PUT /api/contacts/{id}", authMiddleware(updateContact))
	m.HandleFunc("DELETE /api/contacts/{id}", authMiddleware(deleteContact))
	return persistMutations(m)
}
func expansionRequest(t *testing.T, h http.Handler, method, path string, body any, version string) *httptest.ResponseRecorder {
	t.Helper()
	u, _ := findUserByEmail("demo@northwind.dev")
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
	if version != "" {
		r.Header.Set("If-Match", version)
	}
	w := httptest.NewRecorder()
	serveFixtureRequest(h, w, r)
	return w
}
func newBareContact(t *testing.T, h http.Handler) Contact {
	t.Helper()
	w := expansionRequest(t, h, "POST", "/api/contacts", map[string]any{"name": "Recoverable client", "email": "recover@example.com", "stage": "Lead"}, "")
	assertStatus(t, w, 201)
	var out struct{ Contact Contact }
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out.Contact
}
func recordVersion(t *testing.T, h http.Handler, id string) string {
	t.Helper()
	w := expansionRequest(t, h, "GET", "/api/records/contacts/"+id, nil, "")
	assertStatus(t, w, 200)
	var out struct{ Record map[string]any }
	json.Unmarshal(w.Body.Bytes(), &out)
	return fmtString(out.Record["_revision"])
}
func TestRecordVersionRejectsStaleEditsAndDeletes(t *testing.T) {
	setupStore(t)
	h := expansionTestMux()
	c := newBareContact(t, h)
	version := recordVersion(t, h, c.ID)
	changed := map[string]any{"name": "Changed client", "email": c.Email, "stage": "Lead"}
	assertStatus(t, expansionRequest(t, h, "PUT", "/api/contacts/"+c.ID, changed, version), 200)
	assertStatus(t, expansionRequest(t, h, "PUT", "/api/contacts/"+c.ID, map[string]any{"name": "Stale client", "email": c.Email, "stage": "Lead"}, version), 409)
	assertStatus(t, expansionRequest(t, h, "DELETE", "/api/contacts/"+c.ID, nil, version), 409)
	w := expansionRequest(t, h, "GET", "/api/records/contacts/"+c.ID, nil, "")
	if !strings.Contains(w.Body.String(), "Changed client") {
		t.Fatal("stale update overwrote current data")
	}
}

func TestStudentListFiltersLinkedAgentAndSchool(t *testing.T) {
	setupStore(t)
	h := expansionTestMux()
	for _, tc := range []struct{ query, want, omit string }{
		{"agentId=ag_1&limit=200", `"id":"st_1"`, `"id":"st_2"`},
		{"schoolId=s_1&limit=200", `"id":"st_3"`, `"id":"st_4"`},
	} {
		w := expansionRequest(t, h, "GET", "/api/students?"+tc.query, nil, "")
		assertStatus(t, w, 200)
		if !strings.Contains(w.Body.String(), tc.want) || strings.Contains(w.Body.String(), tc.omit) {
			t.Fatalf("relationship filter %q returned unexpected records: %s", tc.query, w.Body.String())
		}
	}
}

func TestAgentAndSchoolCountsFollowStudentLinks(t *testing.T) {
	setupStore(t)
	h := expansionTestMux()
	assertCount := func(path, field string, want int) {
		t.Helper()
		w := expansionRequest(t, h, "GET", path, nil, "")
		assertStatus(t, w, 200)
		var result map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		var record map[string]any
		if rows, ok := result["agents"].([]any); ok && len(rows) > 0 {
			record = rows[0].(map[string]any)
		} else if rows, ok := result["schools"].([]any); ok && len(rows) > 0 {
			record = rows[0].(map[string]any)
		} else if value, ok := result["agent"].(map[string]any); ok {
			record = value
		} else if value, ok := result["school"].(map[string]any); ok {
			record = value
		}
		if record == nil || int(record[field].(float64)) != want {
			t.Fatalf("%s expected %s=%d: %s", path, field, want, w.Body.String())
		}
	}
	assertCount("/api/agents?q=00001", "studentsReferred", 2)
	assertCount("/api/schools?q=toronto", "studentsEnrolled", 2)
	assertStatus(t, expansionRequest(t, h, "POST", "/api/students", map[string]any{
		"name": "New link", "schoolId": "s_1", "agentId": "ag_1",
		"acceptanceStatus": "Pending", "visaStatus": "Not Started", "invoiceStatus": "Not Issued",
	}, ""), 201)
	assertCount("/api/agents/ag_1", "studentsReferred", 3)
	assertCount("/api/schools/s_1", "studentsEnrolled", 3)
}

func TestSearchQueriesFilterCoreCRMLists(t *testing.T) {
	setupStore(t)
	h := expansionTestMux()
	for _, tc := range []struct{ path, id string }{
		{"/api/contacts?q=analytical", `"id":"c_1"`},
		{"/api/deals?q=cryptography", `"id":"d_3"`},
		{"/api/tasks?q=whitepaper", `"id":"t_3"`},
		{"/api/schools?q=toronto", `"id":"s_1"`},
		{"/api/students?q=ecc-ng-2026", `"id":"st_2"`},
		{"/api/agents?q=00003", `"id":"ag_3"`},
		{"/api/leads?q=carlos", `"email":"carlos.reyes@example.com"`},
		{"/api/cases?q=ecc-case-2026-00002", `"id":"case_2"`},
		{"/api/invoices?q=ecc-inv-2026-00001", `"id":"inv_1"`},
	} {
		w := expansionRequest(t, h, "GET", tc.path, nil, "")
		if w.Code != 200 {
			t.Fatalf("search %q returned %d: %s", tc.path, w.Code, w.Body.String())
		}
		if !strings.Contains(strings.ToLower(w.Body.String()), strings.ToLower(tc.id)) || !strings.Contains(w.Body.String(), `"total":1`) {
			t.Fatalf("search %q did not return one matching record: %s", tc.path, w.Body.String())
		}
	}
}
func TestTrashRestoresDeletionBatchAndRejectsNewerRelatedChanges(t *testing.T) {
	setupStore(t)
	h := expansionTestMux()
	c := newBareContact(t, h)
	u, _ := findUserByEmail("demo@northwind.dev")
	mu.Lock()
	tasks = append(tasks, Task{RecordScope: c.RecordScope, ID: "task_recovery", ContactID: c.ID, Title: "Keep this task", Status: "todo"})
	deals = append(deals, Deal{RecordScope: c.RecordScope, ID: "deal_recovery", ContactID: c.ID, Title: "Keep this link", Stage: "Lead", CreatedAt: "2026-10-01", Owner: u.Name})
	mu.Unlock()
	if err := saveStore(); err != nil {
		t.Fatal(err)
	}
	assertStatus(t, expansionRequest(t, h, "DELETE", "/api/contacts/"+c.ID, nil, ""), 200)
	w := expansionRequest(t, h, "GET", "/api/trash", nil, "")
	assertStatus(t, w, 200)
	var out struct{ Batches []struct{ ID string } }
	json.Unmarshal(w.Body.Bytes(), &out)
	if len(out.Batches) != 1 {
		t.Fatal(w.Body.String())
	}
	id := out.Batches[0].ID
	mu.Lock()
	for i := range deals {
		if deals[i].ID == "deal_recovery" {
			deals[i].Title = "Newer change"
		}
	}
	mu.Unlock()
	assertStatus(t, expansionRequest(t, h, "POST", "/api/trash/"+id+"/restore", map[string]any{}, ""), 409)
	mu.Lock()
	for i := range deals {
		if deals[i].ID == "deal_recovery" {
			deals[i].Title = "Keep this link"
		}
	}
	mu.Unlock()
	assertStatus(t, expansionRequest(t, h, "POST", "/api/trash/"+id+"/restore", map[string]any{}, ""), 200)
	s := snapshot()
	foundTask, foundLink := false, false
	for _, task := range s.Tasks {
		foundTask = foundTask || task.ID == "task_recovery"
	}
	for _, deal := range s.Deals {
		foundLink = foundLink || (deal.ID == "deal_recovery" && deal.ContactID == c.ID)
	}
	if !foundTask || !foundLink {
		t.Fatal("restoration lost related records")
	}
	assertStatus(t, expansionRequest(t, h, "POST", "/api/trash/"+id+"/restore", map[string]any{}, ""), 404)
}
func TestSQLReadsBypassLegacyLockAndKeepOrganizationIsolation(t *testing.T) {
	setupStore(t)
	h := expansionTestMux()
	c := newBareContact(t, h)
	mutationMu.Lock()
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- expansionRequest(t, h, "GET", "/api/records/contacts/"+c.ID, nil, "") }()
	select {
	case w := <-done:
		mutationMu.Unlock()
		assertStatus(t, w, 200)
	case <-time.After(2 * time.Second):
		mutationMu.Unlock()
		<-done
		t.Fatal("SQL record reads must not wait on the legacy mutation lock")
	}
	mu.Lock()
	foreign := c
	foreign.ID = "foreign_record"
	foreign.OrgID = "foreign_org"
	contacts = append(contacts, foreign)
	mu.Unlock()
	if err := saveStore(); err != nil {
		t.Fatal(err)
	}
	assertStatus(t, expansionRequest(t, h, "GET", "/api/records/contacts/foreign_record", nil, ""), 404)
}
func TestReportingUsesAllRecordsAndCohortDates(t *testing.T) {
	setupStore(t)
	u, _ := findUserByEmail("demo@northwind.dev")
	scope := RecordScope{OrgID: u.OrgID, OwnerID: u.ID, Visibility: "organization"}
	mu.Lock()
	deals = nil
	for i := 0; i < 501; i++ {
		deals = append(deals, Deal{RecordScope: scope, ID: newID("deal"), Stage: "Won", Value: 1, Currency: "USD", CreatedAt: "2026-01-01"})
	}
	leads = []Lead{{RecordScope: scope, ID: "lead_cohort", Source: "Website", Status: "Converted", CreatedAt: "2026-01-05"}, {RecordScope: scope, ID: "lead_other", Source: "Website", Status: "New", CreatedAt: "2025-01-05"}}
	mu.Unlock()
	if err := saveStore(); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/api/reports", nil)
	request = request.WithContext(context.WithValue(request.Context(), requestUserKey, u))
	handleReports(r, request)
	var totals reportSummary
	json.Unmarshal(r.Body.Bytes(), &totals)
	if totals.StageCounts["Won"] != 501 || totals.CreatedCounts["2026-01-01"] < 501 {
		t.Fatal("dashboard depended on a frontend page")
	}
	w := expansionRequest(t, expansionTestMux(), "GET", "/api/insights?from=2026-01-01&to=2026-12-31", nil, "")
	assertStatus(t, w, 200)
	var out struct{ Sources []conversionMetric }
	json.Unmarshal(w.Body.Bytes(), &out)
	if len(out.Sources) != 1 || out.Sources[0].Leads != 1 || out.Sources[0].Rate != 100 {
		t.Fatal(w.Body.String())
	}
	assertStatus(t, expansionRequest(t, expansionTestMux(), "GET", "/api/insights?from=invalid", nil, ""), 400)
}
func TestPortalCredentialsAreSingleUseScopedAndRevocable(t *testing.T) {
	setupStore(t)
	h := expansionTestMux()
	c := newBareContact(t, h)
	w := expansionRequest(t, h, "POST", "/api/portal/invitations", map[string]string{"contactId": c.ID, "email": c.Email}, "")
	assertStatus(t, w, 201)
	var invite struct{ ID, URL string }
	json.Unmarshal(w.Body.Bytes(), &invite)
	token := strings.Split(invite.URL, "#invite=")[1]
	w = featureRequest(h, "POST", "/api/portal/session", map[string]string{"token": token}, nil)
	assertStatus(t, w, 200)
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly {
		t.Fatal("portal sessions must be HttpOnly")
	}
	assertStatus(t, featureRequest(h, "POST", "/api/portal/session", map[string]string{"token": token}, nil), 401)
	assertStatus(t, featureRequest(h, "POST", "/api/portal/session", map[string]string{"token": cookies[0].Value}, nil), 401)
	assertStatus(t, featureRequest(h, "GET", "/api/contacts/"+c.ID, nil, cookies), 401)
	w = featureRequest(h, "GET", "/api/portal/me", nil, cookies)
	assertStatus(t, w, 200)
	if strings.Contains(w.Body.String(), "Ada Lovelace") || strings.Contains(w.Body.String(), "notes") {
		t.Fatal("portal exposed unrelated/internal records")
	}
	assertStatus(t, featureRequest(h, "POST", "/api/portal/documents/doc_1/files", nil, cookies), 404)
	assertStatus(t, featureRequest(h, "POST", "/api/portal/messages", map[string]string{"caseId": "case_1", "body": "foreign"}, cookies), 404)
	assertStatus(t, expansionRequest(t, h, "DELETE", "/api/portal/invitations/"+invite.ID, nil, ""), 200)
	assertStatus(t, featureRequest(h, "GET", "/api/portal/me", nil, cookies), 401)
}

func TestFollowupRulePreviewActivationAndIdempotentTaskCreation(t *testing.T) {
	setupStore(t)
	h := expansionTestMux()
	rule := map[string]any{"name": "Stale case follow-up", "entity": "cases", "stage": "", "inactiveDays": 1, "taskTitle": "Follow up: {{record}}", "description": "Check the application with the client.", "enabled": true}
	w := expansionRequest(t, h, "POST", "/api/follow-up-rules/preview", rule, "")
	assertStatus(t, w, 200)
	var preview struct {
		Count            int    `json:"count"`
		PreviewToken     string `json:"previewToken"`
		PreviewExpiresAt string `json:"previewExpiresAt"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &preview); err != nil || preview.Count == 0 || preview.PreviewToken == "" {
		t.Fatalf("expected stale cases in preview, got %s", w.Body.String())
	}
	rule["previewToken"] = preview.PreviewToken
	rule["previewExpiresAt"] = preview.PreviewExpiresAt
	rule["description"] = "Changed after preview"
	assertStatus(t, expansionRequest(t, h, "POST", "/api/follow-up-rules", rule, ""), 409)
	rule["description"] = "Check the application with the client."
	w = expansionRequest(t, h, "POST", "/api/follow-up-rules", rule, "")
	assertStatus(t, w, 201)
	var saved struct {
		ID string `json:"id"`
	}
	json.Unmarshal(w.Body.Bytes(), &saved)
	var enabledRule struct {
		Data string `json:"data"`
	}
	database.QueryRow("SELECT data FROM workspace_entries WHERE id=?", saved.ID).Scan(&enabledRule.Data)
	var config followupRule
	json.Unmarshal([]byte(enabledRule.Data), &config)
	if config.Enabled {
		t.Fatal("new rules must remain disabled until a preview is reviewed")
	}
	assertStatus(t, expansionRequest(t, h, "PATCH", "/api/follow-up-rules/"+saved.ID, map[string]any{"enabled": true, "previewToken": "stale-preview", "previewExpiresAt": preview.PreviewExpiresAt}, ""), 409)
	assertStatus(t, expansionRequest(t, h, "PATCH", "/api/follow-up-rules/"+saved.ID, map[string]any{"enabled": true, "previewToken": preview.PreviewToken, "previewExpiresAt": preview.PreviewExpiresAt}, ""), 200)
	before := len(snapshot().Tasks)
	commitFixtureEdits()
	runFollowupRules()
	reloadFixtureAssertions()
	after := len(snapshot().Tasks)
	if after <= before {
		t.Fatal("enabled rule did not create follow-up tasks")
	}
	commitFixtureEdits()
	runFollowupRules()
	reloadFixtureAssertions()
	if len(snapshot().Tasks) != after {
		t.Fatal("repeated automation created duplicate tasks")
	}
	mu.Lock()
	documents[0].Status = "Expired"
	cases[0].StudentID = students[0].ID
	cases[0].CurrentStage = "Approved"
	students[0].ApplicationStage = "Applying"
	mu.Unlock()
	w = expansionRequest(t, h, "GET", "/api/data-quality", nil, "")
	assertStatus(t, w, 200)
	if !strings.Contains(w.Body.String(), "Required document expired") {
		t.Fatal("data quality scan missed an expired required document")
	}
	if strings.Contains(w.Body.String(), students[0].Name+" has no active linked case") {
		t.Fatal("data quality scan flagged a student who has a completed linked case")
	}
	w = expansionRequest(t, h, "GET", "/api/data-quality?severity=error&page=1&pageSize=1", nil, "")
	assertStatus(t, w, 200)
	var page struct {
		Findings []qualityFinding `json:"findings"`
		Total    int              `json:"total"`
		Pages    int              `json:"pages"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil || len(page.Findings) > 1 || page.Pages < 1 || page.Total < len(page.Findings) {
		t.Fatalf("quality filters and pagination should be returned: %s", w.Body.String())
	}
	assertStatus(t, expansionRequest(t, h, "GET", "/api/application-queue", nil, ""), 200)
	assertStatus(t, expansionRequest(t, h, "GET", "/api/admin/operations", nil, ""), 200)
	history := expansionRequest(t, h, "GET", "/api/follow-up-history", nil, "")
	assertStatus(t, history, 200)
	if !strings.Contains(history.Body.String(), "Stale case follow-up") || !strings.Contains(history.Body.String(), "taskId") {
		t.Fatalf("expected generated-task attribution in history, got %s", history.Body.String())
	}
	assertStatus(t, expansionRequest(t, h, "PUT", "/api/follow-up-rules/"+saved.ID, map[string]any{"name": "Edited rule", "entity": "cases", "inactiveDays": 3, "taskTitle": "Follow {{record}}", "description": "Review"}, ""), 200)
	assertStatus(t, expansionRequest(t, h, "DELETE", "/api/follow-up-rules/"+saved.ID, nil, ""), 200)
}

func TestSignedInboundEmailDeduplicatesAndPreservesContactScope(t *testing.T) {
	setupStore(t)
	t.Setenv("INBOUND_EMAIL_SECRET", "inbound-test-secret")
	h := expansionTestMux()
	c := newBareContact(t, h)
	payload := []byte(mustJSON(inboundMessage{ID: "provider-1", OrgID: c.OrgID, From: c.Email, Subject: "Reply", Body: "Please review"}))
	send := func(signature string) *httptest.ResponseRecorder { return signedEmailRequest(h, payload, signature) }
	assertStatus(t, send("invalid"), 401)
	before := len(snapshot().Activities)
	assertStatus(t, send("valid"), 202)
	assertStatus(t, send("valid"), 202)
	s := snapshot()
	if len(s.Activities) != before+1 {
		t.Fatal("provider retry duplicated activities")
	}
	a := s.Activities[len(s.Activities)-1]
	if a.ContactID != c.ID || a.RecordScope != c.RecordScope {
		t.Fatal("incoming mail widened record visibility")
	}
	if a.Kind != "email" {
		t.Fatalf("incoming email has activity kind %q", a.Kind)
	}
	if !isValidActivityKind("whatsapp") {
		t.Fatal("WhatsApp activity kind should be supported")
	}
}
func signedEmailRequest(h http.Handler, payload []byte, signature string) *httptest.ResponseRecorder {
	stamp := strconv.FormatInt(time.Now().Unix(), 10)
	r := httptest.NewRequest("POST", "/api/inbound/email", bytes.NewReader(payload))
	r.Header.Set("X-ECC-Timestamp", stamp)
	if signature == "valid" {
		mac := hmac.New(sha256.New, []byte("inbound-test-secret"))
		mac.Write(append([]byte(stamp+"."), payload...))
		signature = hex.EncodeToString(mac.Sum(nil))
	}
	r.Header.Set("X-ECC-Signature", signature)
	w := httptest.NewRecorder()
	serveFixtureRequest(h, w, r)
	return w
}

type testTransport func(*http.Request) (*http.Response, error)

func (f testTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestAssistantUsesResponsesWithoutStorageAndRequiresActivation(t *testing.T) {
	setupStore(t)
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_MODEL", "configured-model")
	h := expansionTestMux()
	c := newBareContact(t, h)
	assertStatus(t, expansionRequest(t, h, "POST", "/api/assistant", map[string]string{"entity": "contacts", "id": c.ID, "action": "summary"}, ""), 503)
	assertStatus(t, expansionRequest(t, h, "PUT", "/api/admin/assistant", map[string]bool{"enabled": true}, ""), 200)
	w := expansionRequest(t, h, "POST", "/api/assistant", map[string]string{"entity": "contacts", "id": c.ID, "action": "summary"}, "")
	assertStatus(t, w, 202)
	var queued struct{ ID string }
	json.Unmarshal(w.Body.Bytes(), &queued)
	original := providerHTTP
	t.Cleanup(func() { providerHTTP = original })
	providerHTTP = &http.Client{Transport: testTransport(func(r *http.Request) (*http.Response, error) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if r.URL.Path != "/v1/responses" || body["store"] != false || body["model"] != "configured-model" {
			t.Fatal("incorrect provider request")
		}
		if instructions, _ := body["instructions"].(string); strings.Contains(instructions, "daily work brief") {
			input := mustJSON(body["input"])
			if !strings.Contains(input, "Prepare applicant file") || strings.Contains(input, c.Email) || strings.Contains(input, "task_ai_due") {
				t.Fatal("productivity input should be minimized to work facts, without contact details or internal IDs")
			}
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"status":"completed","output":[{"content":[{"type":"output_text","text":"Reviewable summary"}]}]}`)), Header: make(http.Header)}, nil
	})}
	processAssistantJobs(context.Background())
	w = expansionRequest(t, h, "GET", "/api/assistant/jobs/"+queued.ID, nil, "")
	assertStatus(t, w, 200)
	if !strings.Contains(w.Body.String(), "Reviewable summary") || strings.Contains(w.Body.String(), `"input"`) {
		t.Fatal(w.Body.String())
	}
	u, _ := findUserByEmail("demo@northwind.dev")
	today, _ := time.Parse("2006-01-02", organizationTodayForOrg(u.OrgID))
	yesterday := today.AddDate(0, 0, -1).Format("2006-01-02")
	mu.Lock()
	tasks = append(tasks, Task{RecordScope: RecordScope{OrgID: u.OrgID, OwnerID: u.ID, Visibility: "private"}, ID: "task_ai_due", Title: "Prepare applicant file", Description: "Review requested paperwork", Status: "todo", DueDate: yesterday, Owner: u.Name})
	mu.Unlock()
	w = expansionRequest(t, h, "POST", "/api/assistant/productivity", nil, "")
	assertStatus(t, w, 202)
	var briefQueued struct {
		ID    string             `json:"id"`
		Items []productivityItem `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &briefQueued); err != nil || len(briefQueued.Items) == 0 {
		t.Fatalf("work brief should put the user's overdue task first: %s", w.Body.String())
	}
	foundOverdue := false
	for _, item := range briefQueued.Items {
		if item.ID == "task_ai_due" && item.Priority == "urgent" {
			foundOverdue = true
		}
	}
	if !foundOverdue {
		t.Fatalf("overdue task should appear with urgent priority: %s", w.Body.String())
	}
	processAssistantJobs(context.Background())
	w = expansionRequest(t, h, "GET", "/api/assistant/jobs/"+briefQueued.ID, nil, "")
	assertStatus(t, w, 200)
	if !strings.Contains(w.Body.String(), "Reviewable summary") || strings.Contains(w.Body.String(), "task_ai_due") || strings.Contains(w.Body.String(), `"input"`) {
		t.Fatalf("completed job should discard inputs and keep record IDs out of the AI job response: %s", w.Body.String())
	}
	var count int
	database.QueryRow("SELECT count(*) FROM mail_outbox WHERE contact_id=?", c.ID).Scan(&count)
	if count != 0 {
		t.Fatal("AI must never send a message automatically")
	}
}

func TestSQLReadsUseCommittedRolesEvenWhenCacheContainsUncommittedRoleChange(t *testing.T) {
	setupStore(t)
	h := expansionTestMux()
	u, _ := findUserByEmail("demo@northwind.dev")
	// Commit a member role, then simulate the temporary cache state of an admin grant.
	u.Role = "member"
	mu.Lock()
	users[u.Email] = u
	private := Contact{RecordScope: RecordScope{OrgID: u.OrgID, OwnerID: "another_user", Visibility: "private"}, ID: "private_other", Name: "Restricted", Stage: "Lead"}
	contacts = append(contacts, private)
	mu.Unlock()
	if err := saveStore(); err != nil {
		t.Fatal(err)
	}
	u.Role = "admin"
	mu.Lock()
	users[u.Email] = u
	mu.Unlock()
	assertStatus(t, expansionRequest(t, h, "GET", "/api/records/contacts/private_other", nil, ""), 404)
}

func TestPureRecordFingerprintsIgnoreOnlyRevisionMetadata(t *testing.T) {
	a := map[string]any{"id": "record", "visibility": "private", "name": "Client", "balance": 10.0}
	b := map[string]any{"name": "Client", "visibility": "private", "id": "record", "balance": 10.0, "_revision": "old"}
	if versionOf(a) != versionOf(b) {
		t.Fatal("fingerprint depends on response metadata or map order")
	}
	b["name"] = "Changed"
	if versionOf(a) == versionOf(b) {
		t.Fatal("fingerprint ignored a record change")
	}
	result := withRecordVersions(map[string]any{"records": []any{a}, "user": map[string]any{"id": "staff", "email": "staff@example.com"}}).(map[string]any)
	if fmtString(result["records"].([]any)[0].(map[string]any)["_revision"]) == "" {
		t.Fatal("nested records lack versions")
	}
	if _, ok := result["user"].(map[string]any)["_revision"]; ok {
		t.Fatal("account is not a versioned CRM record")
	}
}
func TestPureCohortMetricsUseExplicitIntakesAndEnrolmentStates(t *testing.T) {
	s := diskStore{Leads: []Lead{{Source: "Website", Status: "Converted", CreatedAt: "2026-02-01"}, {Source: "Website", Status: "New", CreatedAt: "2026-02-02"}, {Source: "Referral", Status: "Converted", CreatedAt: "2025-01-01"}}, Students: []Student{{SchoolID: "school", SchoolName: "College", AgentID: "agent", AgentName: "Adviser", Intake: "September 2026", ApplicationStage: "Enrolled", CreatedAt: "2026-02-01"}, {SchoolID: "school", SchoolName: "College", ApplicationStage: "Accepted", CreatedAt: "2026-02-02"}}}
	sources, schools, agents := cohortMetrics(s, "2026-01-01", "2026-12-31")
	if len(sources) != 1 || sources[0].Leads != 2 || sources[0].Converted != 1 || sources[0].Rate != 50 {
		t.Fatalf("incorrect conversion: %+v", sources)
	}
	if len(schools) != 2 || schools[0].Intake != "September 2026" || schools[0].Enrolled != 1 || schools[1].Intake != "Unspecified" || schools[1].Enrolled != 0 {
		t.Fatalf("incorrect enrolment grouping: %+v", schools)
	}
	if len(agents) != 2 {
		t.Fatalf("unassigned agent group was dropped: %+v", agents)
	}
}
func TestPureProviderValidationRejectsUnsafeLinksAndInvalidSignatures(t *testing.T) {
	for _, url := range []string{"http://graph.microsoft.com/v1.0/me", "https://evil.example/v1.0/me", "https://graph.microsoft.com.evil.example/v1.0/me", "https://user@graph.microsoft.com/v1.0/me", "https://graph.microsoft.com:443/v1.0/me"} {
		if validGraphLink(url) {
			t.Fatalf("unsafe cursor accepted: %s", url)
		}
	}
	if !validGraphLink("https://graph.microsoft.com/v1.0/me/mailFolders/inbox/messages/delta?$deltatoken=token") {
		t.Fatal("valid delta link rejected")
	}
	body := []byte("payload")
	mac := hmac.New(sha256.New, []byte("secret"))
	mac.Write(body)
	signature := hex.EncodeToString(mac.Sum(nil))
	if !signedInbound("secret", signature, body) || signedInbound("", signature, body) || signedInbound("secret", signature, []byte("changed")) {
		t.Fatal("signature validation failed")
	}
	if !subjectCaseReference("Re: [ECC-001] Document update", "ECC-001") || subjectCaseReference("ECC-0012", "ECC-001") {
		t.Fatal("case references must match complete tokens")
	}
}
