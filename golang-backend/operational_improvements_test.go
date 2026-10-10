package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTargetedContactEditRetainsScopeRevisionsAndRelatedRecords(t *testing.T) {
	setupStore(t)
	admin, _ := findUserByEmail("demo@northwind.dev")
	member := addPermissionUser(t, "member", admin.OrgID, "targeted-owner")
	viewer := addPermissionUser(t, "viewer", admin.OrgID, "targeted-viewer")
	mu.Lock()
	contacts[0].OwnerID = member.ID
	contacts[1].Visibility = "private"
	contacts[1].OwnerID = admin.ID
	mu.Unlock()
	if err := saveStore(); err != nil {
		t.Fatal(err)
	}
	var casesBefore, docsBefore string
	database.QueryRow("SELECT data FROM cases ORDER BY id LIMIT 1").Scan(&casesBefore)
	database.QueryRow("SELECT data FROM documents ORDER BY id LIMIT 1").Scan(&docsBefore)
	checked := false
	mux := http.NewServeMux()
	registerWorkspaceRoutes(mux)
	mux.HandleFunc("PUT /api/contacts/{id}", authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		state, done := runtimeForCall(r)
		defer done()
		s := state.snapshot()
		if len(s.Contacts) > 2 || len(s.Cases) > 0 || len(s.Documents) > 0 {
			t.Error("contact edit loaded unrelated workspace data")
		}
		checked = true
		updateContact(w, r)
	}))
	h := persistMutations(mux)
	request := contactRequest{Name: "Targeted edit", Email: contacts[0].Email, Phone: "+63 917 123 4567", Stage: "Lead"}
	assertStatus(t, rawUserRequest(t, h, member, "PUT", "/api/contacts/c_1", request), 200)
	if !checked {
		t.Fatal("contact handler was not called")
	}
	var raw string
	database.QueryRow("SELECT data FROM contacts WHERE id='c_1'").Scan(&raw)
	var c Contact
	json.Unmarshal([]byte(raw), &c)
	if c.Name != request.Name || c.OwnerID != member.ID || c.Phone != "+639171234567" {
		t.Fatalf("bad scoped update: %+v", c)
	}
	assertStatus(t, rawUserRequest(t, h, viewer, "PUT", "/api/contacts/c_1", request), 403)
	request.Email = contacts[1].Email
	assertStatus(t, rawUserRequest(t, h, admin, "PUT", "/api/contacts/c_1", request), 400)
	// A hidden email collision must not reveal another member's private contact.
	assertStatus(t, rawUserRequest(t, h, member, "PUT", "/api/contacts/c_1", request), 200)
	token, _ := issueToken(member)
	r := httptest.NewRequest("PUT", "/api/contacts/c_1", strings.NewReader(mustJSON(request)))
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("If-Match", "stale")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	assertStatus(t, w, 409)
	database.QueryRow("SELECT data FROM cases ORDER BY id LIMIT 1").Scan(&raw)
	if raw != casesBefore {
		t.Fatal("contact save changed an unrelated case")
	}
	database.QueryRow("SELECT data FROM documents ORDER BY id LIMIT 1").Scan(&raw)
	if raw != docsBefore {
		t.Fatal("contact save changed an unrelated document")
	}
}

func TestReceiptDuplicatesApprovalAndPostedBalance(t *testing.T) {
	setupStore(t)
	admin, _ := findUserByEmail("demo@northwind.dev")
	member := addPermissionUser(t, "member", admin.OrgID, "receipt-owner")
	foreign := addPermissionUser(t, "admin", "receipt-foreign", "receipt-foreign-admin")
	scope := RecordScope{OrgID: admin.OrgID, OwnerID: member.ID, Visibility: "organization"}
	mu.Lock()
	invoices = []Invoice{{RecordScope: scope, ID: "receipt-invoice", InvoiceNumber: "RECEIPT-TEST", ClientID: "c_1", CaseID: cases[0].ID, TotalFee: 100, Balance: 100, LedgerInitialized: true, Currency: "USD"}}
	payments = nil
	mu.Unlock()
	if err := saveStore(); err != nil {
		t.Fatal(err)
	}
	h := featureMux()
	create := func(req paymentRequest, status int) Payment {
		t.Helper()
		w := rawUserRequest(t, h, member, "POST", "/api/payments", req)
		assertStatus(t, w, status)
		var out struct{ Payment Payment }
		json.Unmarshal(w.Body.Bytes(), &out)
		return out.Payment
	}
	req := paymentRequest{InvoiceID: "receipt-invoice", Amount: 50, PaidOn: "2026-10-10", Method: "Bank", Reference: "receipt-one", RequireApproval: true}
	first := create(req, 201)
	finance := func(balance, amount float64, count int) {
		t.Helper()
		w := rawUserRequest(t, h, admin, "GET", "/api/invoices/receipt-invoice/finance-status", nil)
		assertStatus(t, w, 200)
		var out struct {
			Invoice       Invoice
			PendingAmount float64
			PendingCount  int
		}
		json.Unmarshal(w.Body.Bytes(), &out)
		if out.Invoice.Balance != balance || out.PendingAmount != amount || out.PendingCount != count {
			t.Fatalf("incorrect approval balance: %s", w.Body.String())
		}
	}
	finance(100, 50, 1)
	req.ConfirmDuplicate = true
	create(req, 400) // Exact reference cannot be overridden.
	req.Reference = "receipt-two"
	req.ConfirmDuplicate = false
	create(req, 400)
	req.ConfirmDuplicate = true
	second := create(req, 201)
	finance(100, 100, 2)
	assertStatus(t, rawUserRequest(t, h, member, "POST", "/api/payments/"+first.ID+"/review", map[string]string{"decision": "approve"}), 403)
	assertStatus(t, rawUserRequest(t, h, foreign, "POST", "/api/payments/"+first.ID+"/review", map[string]string{"decision": "approve"}), 404)
	assertStatus(t, rawUserRequest(t, h, admin, "DELETE", "/api/payments/"+first.ID, nil), 409)
	assertStatus(t, rawUserRequest(t, h, admin, "POST", "/api/payments/"+first.ID+"/review", map[string]string{"decision": "approve"}), 200)
	finance(50, 50, 1)
	assertStatus(t, rawUserRequest(t, h, admin, "DELETE", "/api/payments/"+first.ID, nil), 409)
	assertStatus(t, rawUserRequest(t, h, admin, "POST", "/api/payments/"+first.ID+"/review", map[string]string{"decision": "approve"}), 409)
	assertStatus(t, rawUserRequest(t, h, admin, "POST", "/api/payments/"+second.ID+"/review", map[string]string{"decision": "reject"}), 200)
	finance(50, 0, 0)
	var reviews int
	database.QueryRow("SELECT count(*) FROM workspace_entries WHERE category='receipt_review'").Scan(&reviews)
	if reviews != 2 {
		t.Fatal("receipt review audit missing")
	}
}

func TestApplicationBlockersKeepPrivateDocumentsHidden(t *testing.T) {
	setupStore(t)
	admin, _ := findUserByEmail("demo@northwind.dev")
	member := addPermissionUser(t, "member", admin.OrgID, "blocker-reader")
	scope := RecordScope{OrgID: admin.OrgID, OwnerID: member.ID, Visibility: "organization"}
	hidden := scope
	hidden.OwnerID = admin.ID
	hidden.Visibility = "private"
	mu.Lock()
	cases = []Case{{RecordScope: scope, ID: "blocker-case", AutoAdvance: true, CurrentStage: "Documents Pending", CaseNumber: "BLOCKERS"}}
	documents = []Document{{RecordScope: scope, ID: "blocker-doc", CaseID: "blocker-case", DocName: "Passport", Required: true, Status: "Requested"}, {RecordScope: hidden, ID: "private-doc", CaseID: "blocker-case", DocName: "Secret medical document", Required: true, Status: "Requested"}}
	invoices = []Invoice{{RecordScope: scope, ID: "blocker-invoice", CaseID: "blocker-case", InvoiceNumber: "BLOCK-INV", TotalFee: 100, Balance: 100, LedgerInitialized: true, Currency: "USD"}}
	payments = []Payment{{RecordScope: scope, ID: "blocker-receipt", InvoiceID: "blocker-invoice", Amount: 25, Status: "pending_approval"}}
	mu.Unlock()
	if err := saveStore(); err != nil {
		t.Fatal(err)
	}
	h := expansionTestMux()
	w := rawUserRequest(t, h, member, "GET", "/api/cases/blocker-case/blockers", nil)
	assertStatus(t, w, 200)
	var out struct {
		Items         []ApplicationBlocker
		BlockingCount int
	}
	json.Unmarshal(w.Body.Bytes(), &out)
	if out.BlockingCount != 1 || len(out.Items) != 3 {
		t.Fatalf("wrong visible requirements: %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "Secret") || strings.Contains(w.Body.String(), "private-doc") {
		t.Fatal("private requirements leaked")
	}
	if out.Items[0].Route != "/cases/blocker-case#record-tools?tab=documents&document=blocker-doc" {
		t.Fatal("document action did not target the document")
	}
	for _, item := range out.Items[1:] {
		if item.Blocking {
			t.Fatal("finance attention was misrepresented as a stage gate")
		}
	}
}

func TestHandoffRemindersAreDailyAndStopWhenResolved(t *testing.T) {
	setupStore(t)
	admin, _ := findUserByEmail("demo@northwind.dev")
	recipient := addPermissionUser(t, "member", admin.OrgID, "reminder-recipient")
	h := expansionTestMux()
	w := rawUserRequest(t, h, admin, "POST", "/api/cases/"+cases[0].ID+"/handoffs", map[string]string{"toId": recipient.ID, "reason": "Review evidence", "nextAction": "Check passport", "dueDate": "2099-01-01"})
	assertStatus(t, w, 201)
	var out struct{ Handoff ApplicationHandoff }
	json.Unmarshal(w.Body.Bytes(), &out)
	out.Handoff.DueDate = "2000-01-01"
	out.Handoff.CreatedAt = time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339)
	database.Exec("UPDATE workspace_entries SET data=? WHERE id=?", mustJSON(out.Handoff), out.Handoff.ID)
	generateReminders()
	generateReminders()
	var count int
	database.QueryRow("SELECT count(*) FROM notifications WHERE dedupe_key LIKE 'handoff-reminder:%'").Scan(&count)
	if count != 2 {
		t.Fatalf("expected one reminder per staff member: %d", count)
	}
	queue := rawUserRequest(t, h, recipient, "GET", "/api/my-work", nil)
	assertStatus(t, queue, 200)
	if !strings.Contains(queue.Body.String(), "Overdue handoff") || !strings.Contains(queue.Body.String(), `"overdueHandoffs":1`) {
		t.Fatal("overdue handoff absent from daily work")
	}
	assertStatus(t, rawUserRequest(t, h, admin, "POST", "/api/handoffs/"+out.Handoff.ID+"/cancel", nil), 200)
	generateReminders()
	database.QueryRow("SELECT count(*) FROM notifications WHERE dedupe_key LIKE 'handoff-reminder:%' AND read_at IS NULL").Scan(&count)
	if count != 0 {
		t.Fatal("resolved handoff still has outstanding reminders")
	}
}

func TestScopedCaseStageTrackingRecordsEachTransitionOnce(t *testing.T) {
	setupStore(t)
	admin, _ := findUserByEmail("demo@northwind.dev")
	id := cases[0].ID
	h := featureMux()
	var initial int
	database.QueryRow("SELECT count(*) FROM case_stage_events WHERE case_id=?", id).Scan(&initial)
	for _, stage := range []string{"Submitted", "Submitted", "Decision Pending"} {
		assertStatus(t, rawUserRequest(t, h, admin, "PATCH", "/api/cases/"+id+"/stage", map[string]string{"currentStage": stage}), 200)
	}
	var count int
	database.QueryRow("SELECT count(*) FROM case_stage_events WHERE case_id=?", id).Scan(&count)
	if count != initial+2 {
		t.Fatalf("missing or duplicate transitions: %d -> %d", initial, count)
	}
	var stage string
	database.QueryRow("SELECT stage FROM case_stage_events WHERE case_id=? ORDER BY rowid DESC LIMIT 1", id).Scan(&stage)
	if stage != "Decision Pending" {
		t.Fatal("stage history did not follow persisted case changes")
	}
}

func TestReceiptControlsIncludeHiddenLedgerWithoutLeakingReceiptDetails(t *testing.T) {
	setupStore(t)
	admin, _ := findUserByEmail("demo@northwind.dev")
	member := addPermissionUser(t, "member", admin.OrgID, "hidden-receipt-reader")
	scope := RecordScope{OrgID: admin.OrgID, OwnerID: member.ID, Visibility: "organization"}
	hidden := scope
	hidden.OwnerID = admin.ID
	hidden.Visibility = "private"
	mu.Lock()
	invoices = []Invoice{{RecordScope: scope, ID: "hidden-ledger-invoice", InvoiceNumber: "HIDDEN-LEDGER", ClientID: "c_1", TotalFee: 100, Balance: 100, LedgerInitialized: true, Currency: "USD"}}
	payments = []Payment{{RecordScope: hidden, ID: "private-receipt-id", InvoiceID: "hidden-ledger-invoice", Amount: 20, Status: "pending_approval", PaidOn: "2026-10-10", Method: "Bank", Reference: "private-reference", CreatedBy: "Private receipt owner"}}
	mu.Unlock()
	if err := saveStore(); err != nil {
		t.Fatal(err)
	}
	h := featureMux()
	w := rawUserRequest(t, h, member, "POST", "/api/payments", paymentRequest{InvoiceID: "hidden-ledger-invoice", Amount: 10, Reference: "private-reference"})
	assertStatus(t, w, 400)
	if strings.Contains(w.Body.String(), "private-receipt-id") || strings.Contains(w.Body.String(), "Private receipt owner") {
		t.Fatal("duplicate warning leaked private receipt details")
	}
	w = rawUserRequest(t, h, member, "GET", "/api/my-work?focus=finance&scope=all", nil)
	assertStatus(t, w, 200)
	if !strings.Contains(w.Body.String(), `"pendingApprovals":1`) {
		t.Fatal("finance queue omitted the pending invoice aggregate")
	}
	w = rawUserRequest(t, h, member, "GET", "/api/invoices/hidden-ledger-invoice/finance-status", nil)
	assertStatus(t, w, 200)
	var out struct {
		PendingCount    int
		PendingReceipts []Payment
	}
	json.Unmarshal(w.Body.Bytes(), &out)
	if out.PendingCount != 1 || len(out.PendingReceipts) != 0 {
		t.Fatal("pending aggregate and private receipt scope were mixed")
	}
}
