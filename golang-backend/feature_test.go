package main

import (
	"archive/zip"
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func featureMux() http.Handler {
	m := http.NewServeMux()
	registerWorkspaceRoutes(m)
	m.HandleFunc("POST /api/login", handleSecureLogin)
	m.HandleFunc("POST /api/register", handleSecureRegister)
	m.HandleFunc("POST /api/session/refresh", handleSessionRefresh)
	m.HandleFunc("POST /api/session/logout", handleSessionLogout)
	m.HandleFunc("GET /api/me", authMiddleware(handleMe))
	m.HandleFunc("POST /api/me/two-factor", authMiddleware(handleTwoFactor))
	m.HandleFunc("DELETE /api/me/sessions/{session}", authMiddleware(handleRevokeSession))
	m.HandleFunc("POST /api/forgot-password", handleForgotPassword)
	m.HandleFunc("POST /api/reset-password", handleResetPassword)
	m.HandleFunc("GET /api/contacts", authMiddleware(listContacts))
	m.HandleFunc("POST /api/contacts", authMiddleware(createContact))
	m.HandleFunc("PUT /api/contacts/{id}", authMiddleware(updateContact))
	m.HandleFunc("GET /api/reports", authMiddleware(handleReports))
	m.HandleFunc("GET /api/search", authMiddleware(handleSearch))
	m.HandleFunc("GET /api/audit", authMiddleware(handleAudit))
	m.HandleFunc("GET /api/exports/{entity}", authMiddleware(handleExport))
	m.HandleFunc("POST /api/payments", authMiddleware(createPaymentHandler))
	m.HandleFunc("DELETE /api/payments/{id}", authMiddleware(deletePaymentHandler))
	m.HandleFunc("PATCH /api/cases/{id}/stage", authMiddleware(updateCaseStageHandler))
	m.HandleFunc("PATCH /api/documents/{id}/status", authMiddleware(updateDocumentStatusHandler))
	return persistMutations(m)
}
func featureRequest(h http.Handler, method, path string, data any, cookies []*http.Cookie) *httptest.ResponseRecorder {
	body := ""
	if data != nil {
		body = mustJSON(data)
	}
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		r.AddCookie(c)
	}
	w := httptest.NewRecorder()
	serveFixtureRequest(h, w, r)
	return w
}
func demoCookies(t *testing.T, h http.Handler) []*http.Cookie {
	t.Helper()
	w := featureRequest(h, "POST", "/api/login", map[string]any{"email": "demo@northwind.dev", "password": "Demo1234", "remember": true}, nil)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	return w.Result().Cookies()
}
func assertStatus(t *testing.T, w *httptest.ResponseRecorder, status int) {
	t.Helper()
	if w.Code != status {
		t.Fatalf("expected %d got %d: %s", status, w.Code, w.Body.String())
	}
}
func TestCookieSessionsRotationAndRevocation(t *testing.T) {
	setupStore(t)
	h := featureMux()
	cookies := demoCookies(t, h)
	if len(cookies) != 2 || !cookies[0].HttpOnly || !cookies[1].HttpOnly {
		t.Fatal("cookies must be HttpOnly")
	}
	w := featureRequest(h, "GET", "/api/me", nil, cookies)
	assertStatus(t, w, 200)
	refresh := featureRequest(h, "POST", "/api/session/refresh", nil, cookies)
	assertStatus(t, refresh, 200)
	assertStatus(t, featureRequest(h, "POST", "/api/session/refresh", nil, cookies), 401)
	assertStatus(t, featureRequest(h, "GET", "/api/me", nil, cookies), 401)
	next := refresh.Result().Cookies()
	assertStatus(t, featureRequest(h, "GET", "/api/me", nil, next), 200)
	assertStatus(t, featureRequest(h, "POST", "/api/session/logout", nil, next), 200)
	assertStatus(t, featureRequest(h, "GET", "/api/me", nil, next), 401)
}
func TestOrganizationsRolesScopesAndAudit(t *testing.T) {
	setupStore(t)
	h := featureMux()
	admin := demoCookies(t, h)
	w := featureRequest(h, "POST", "/api/register", map[string]string{"name": "Other admin", "email": "other@example.com", "password": "LongEnoughPassword!"}, nil)
	assertStatus(t, w, 201)
	other := w.Result().Cookies()
	r := featureRequest(h, "GET", "/api/contacts", nil, other)
	assertStatus(t, r, 200)
	if strings.Contains(r.Body.String(), contacts[0].Name) {
		t.Fatal("cross organization leak")
	}
	r = featureRequest(h, "GET", "/api/search?q=Smith", nil, other)
	if strings.Contains(r.Body.String(), "Smith") {
		t.Fatal("search leak")
	}
	id := contacts[0].ID
	assertStatus(t, featureRequest(h, "PATCH", "/api/access/contacts/"+id, map[string]string{"visibility": "private", "ownerId": "u_1", "teamId": "team_default"}, admin), 200)
	w = featureRequest(h, "POST", "/api/admin/users", map[string]string{"name": "Viewer", "email": "viewer@example.com", "password": "ViewerLongPassword!", "role": "viewer"}, admin)
	assertStatus(t, w, 201)
	w = featureRequest(h, "POST", "/api/login", map[string]string{"email": "viewer@example.com", "password": "ViewerLongPassword!"}, nil)
	assertStatus(t, w, 200)
	viewer := w.Result().Cookies()
	r = featureRequest(h, "GET", "/api/contacts", nil, viewer)
	if strings.Contains(r.Body.String(), id) {
		t.Fatal("private leak")
	}
	r = featureRequest(h, "GET", "/api/audit", nil, viewer)
	if strings.Contains(r.Body.String(), id) {
		t.Fatal("audit leak")
	}
	assertStatus(t, featureRequest(h, "POST", "/api/contacts", map[string]string{"name": "Forbidden"}, viewer), 403)
	assertStatus(t, featureRequest(h, "POST", "/api/admin/users", map[string]string{}, viewer), 403)
	assertStatus(t, featureRequest(h, "PATCH", "/api/admin/users/u_1", map[string]string{"role": "member"}, admin), 409)
}
func TestInvitesResetAndTokenSingleUse(t *testing.T) {
	setupStore(t)
	h := featureMux()
	admin := demoCookies(t, h)
	w := featureRequest(h, "POST", "/api/admin/users", map[string]string{"name": "Invite", "email": "invite@example.com", "role": "member"}, admin)
	assertStatus(t, w, 201)
	var response map[string]any
	json.Unmarshal(w.Body.Bytes(), &response)
	link := response["inviteLink"].(string)
	token := strings.Split(link, "token=")[1]
	assertStatus(t, featureRequest(h, "POST", "/api/invites/accept", map[string]string{"token": token, "password": "InviteLongPassword!"}, nil), 200)
	assertStatus(t, featureRequest(h, "POST", "/api/invites/accept", map[string]string{"token": token, "password": "InviteLongPassword!"}, nil), 400)
	assertStatus(t, featureRequest(h, "POST", "/api/forgot-password", map[string]string{"email": "invite@example.com"}, nil), 200)
	var body string
	database.QueryRow("SELECT body FROM mail_outbox WHERE kind='password_reset'").Scan(&body)
	parts := strings.Split(body, "token=")
	if len(parts) < 2 {
		t.Fatal("reset mail missing")
	}
	token = strings.Fields(parts[1])[0]
	assertStatus(t, featureRequest(h, "POST", "/api/reset-password", map[string]string{"token": token, "password": "ReplacementPassword!"}, nil), 200)
	assertStatus(t, featureRequest(h, "POST", "/api/reset-password", map[string]string{"token": token, "password": "ReplacementPassword!"}, nil), 400)
}
func TestTOTPPersistenceReplayAndRecovery(t *testing.T) {
	setupStore(t)
	h := featureMux()
	cookies := demoCookies(t, h)
	w := featureRequest(h, "POST", "/api/me/two-factor", map[string]string{"action": "setup", "password": "Demo1234"}, cookies)
	assertStatus(t, w, 200)
	var setup map[string]string
	json.Unmarshal(w.Body.Bytes(), &setup)
	code := totpCode(setup["secret"], time.Now().Unix()/30)
	w = featureRequest(h, "POST", "/api/me/two-factor", map[string]string{"action": "confirm", "password": "Demo1234", "code": code}, cookies)
	assertStatus(t, w, 200)
	var confirmed struct{ RecoveryCodes []string }
	json.Unmarshal(w.Body.Bytes(), &confirmed)
	u, _ := findUserByEmail("demo@northwind.dev")
	if verifySecondFactor(&u, code) {
		t.Fatal("TOTP replay allowed")
	}
	if err := loadStore(dataPath); err != nil {
		t.Fatal(err)
	}
	u, _ = findUserByEmail(u.Email)
	if verifySecondFactor(&u, code) {
		t.Fatal("replay counter lost on restart")
	}
	w = featureRequest(h, "POST", "/api/login", map[string]string{"email": u.Email, "password": "Demo1234", "otp": confirmed.RecoveryCodes[0]}, nil)
	assertStatus(t, w, 200)
	w = featureRequest(h, "POST", "/api/login", map[string]string{"email": u.Email, "password": "Demo1234", "otp": confirmed.RecoveryCodes[0]}, nil)
	assertStatus(t, w, 401)
}
func TestLockoutAndCSRF(t *testing.T) {
	setupStore(t)
	h := featureMux()
	for i := 0; i < 5; i++ {
		assertStatus(t, featureRequest(h, "POST", "/api/login", map[string]string{"email": "demo@northwind.dev", "password": "wrong"}, nil), 401)
	}
	assertStatus(t, featureRequest(h, "POST", "/api/login", map[string]string{"email": "demo@northwind.dev", "password": "Demo1234"}, nil), 429)
	r := httptest.NewRequest("POST", "/api/register", strings.NewReader(`{}`))
	r.Header.Set("Origin", "https://evil.example")
	w := httptest.NewRecorder()
	serveFixtureRequest(h, w, r)
	assertStatus(t, w, 403)
}
func multipartRequest(h http.Handler, path, name, filename string, data []byte, cookies []*http.Cookie, fields map[string]string) *httptest.ResponseRecorder {
	var b bytes.Buffer
	m := multipart.NewWriter(&b)
	f, _ := m.CreateFormFile(name, filename)
	f.Write(data)
	for k, v := range fields {
		m.WriteField(k, v)
	}
	m.Close()
	r := httptest.NewRequest("POST", path, &b)
	r.Header.Set("Content-Type", m.FormDataContentType())
	for _, c := range cookies {
		r.AddCookie(c)
	}
	w := httptest.NewRecorder()
	serveFixtureRequest(h, w, r)
	return w
}
func TestProtectedFilesVersionsAndExports(t *testing.T) {
	setupStore(t)
	h := featureMux()
	cookies := demoCookies(t, h)
	id := documents[0].ID
	data := []byte("%PDF-1.4\nfixture")
	for version := 1; version <= 2; version++ {
		w := multipartRequest(h, "/api/documents/"+id+"/files", "file", "passport.pdf", data, cookies, nil)
		assertStatus(t, w, 201)
		if !strings.Contains(w.Body.String(), fmt.Sprintf(`"version":%d`, version)) {
			t.Fatal(w.Body.String())
		}
	}
	w := featureRequest(h, "GET", "/api/documents/"+id+"/files/1", nil, cookies)
	assertStatus(t, w, 200)
	if !bytes.Equal(w.Body.Bytes(), data) {
		t.Fatal("download changed bytes")
	}
	assertStatus(t, featureRequest(h, "GET", "/api/documents/"+id+"/files/1", nil, nil), 401)
	assertStatus(t, multipartRequest(h, "/api/documents/"+id+"/files", "file", "bad.pdf", []byte("<script>alert(1)</script>"), cookies, nil), 400)
	w = featureRequest(h, "GET", "/api/exports/contacts?format=xlsx", nil, cookies)
	assertStatus(t, w, 200)
	z, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if err != nil || len(z.File) != 5 {
		t.Fatal("invalid xlsx")
	}
	w = featureRequest(h, "GET", "/api/invoices/"+invoices[0].ID+"/pdf", nil, cookies)
	assertStatus(t, w, 200)
	if !bytes.HasPrefix(w.Body.Bytes(), []byte("%PDF")) {
		t.Fatal("invalid PDF")
	}
}
func TestImportPreviewAtomicityAndValidation(t *testing.T) {
	setupStore(t)
	h := featureMux()
	cookies := demoCookies(t, h)
	before := len(contacts)
	data := []byte("name,email,phone\nImport Person,import@example.com,+639171234567\nInvalid,bad-email,12\n")
	w := multipartRequest(h, "/api/imports/contacts", "file", "contacts.csv", data, cookies, map[string]string{"preview": "true"})
	assertStatus(t, w, 200)
	if len(contacts) != before {
		t.Fatal("preview mutated records")
	}
	w = multipartRequest(h, "/api/imports/contacts", "file", "contacts.csv", data, cookies, map[string]string{"preview": "false"})
	assertStatus(t, w, 422)
	if len(contacts) != before {
		t.Fatal("partial import")
	}
	data = []byte("name,email,phone\nImport Person,import@example.com,+639171234567\n")
	assertStatus(t, multipartRequest(h, "/api/imports/contacts", "file", "contacts.csv", data, cookies, map[string]string{"preview": "false"}), 200)
	if len(contacts) != before+1 {
		t.Fatal("import missing")
	}
	assertStatus(t, multipartRequest(h, "/api/imports/contacts", "file", "contacts.csv", data, cookies, map[string]string{"preview": "false"}), 422)
}
func TestInvoiceLedgerRefundCreditAndPaymentDeletion(t *testing.T) {
	setupStore(t)
	h := featureMux()
	cookies := demoCookies(t, h)
	inv := invoices[0]
	w := featureRequest(h, "POST", "/api/payments", map[string]any{"invoiceId": inv.ID, "amount": 100, "paidOn": "2026-10-06", "method": "Bank"}, cookies)
	assertStatus(t, w, 201)
	var result struct{ Payment Payment }
	json.Unmarshal(w.Body.Bytes(), &result)
	if invoices[0].AmountReceived != inv.AmountReceived+100 {
		t.Fatal("balance did not increase")
	}
	assertStatus(t, featureRequest(h, "DELETE", "/api/payments/"+result.Payment.ID, nil, cookies), 200)
	if invoices[0].AmountReceived != inv.AmountReceived || invoices[0].Balance != inv.Balance {
		t.Fatal("deleted receipt remains in balance")
	}
	w = featureRequest(h, "POST", "/api/workflows/refund-request", map[string]any{"id": inv.ID, "amount": 50, "reason": "Partial service cancellation"}, cookies)
	assertStatus(t, w, 201)
	var refund map[string]string
	json.Unmarshal(w.Body.Bytes(), &refund)
	assertStatus(t, featureRequest(h, "POST", "/api/workflows/refund-review", map[string]string{"id": refund["id"], "decision": "approved"}, cookies), 200)
	assertStatus(t, featureRequest(h, "POST", "/api/workflows/refund-process", map[string]string{"id": refund["id"], "reference": "BANK-TEST"}, cookies), 200)
	if invoices[0].AmountReceived != inv.AmountReceived-50 {
		t.Fatal("refund missing")
	}
	assertStatus(t, featureRequest(h, "POST", "/api/workflows/refund-process", map[string]string{"id": refund["id"], "reference": "BANK-TEST"}, cookies), 400)
	assertStatus(t, featureRequest(h, "POST", "/api/workflows/refund-reverse", map[string]string{"id": refund["id"], "reference": "REVERSAL-TEST"}, cookies), 200)
	if invoices[0].AmountReceived != inv.AmountReceived {
		t.Fatal("reversal missing")
	}
	assertStatus(t, featureRequest(h, "POST", "/api/workflows/credit-note", map[string]any{"id": inv.ID, "amount": 20, "reason": "Courtesy credit"}, cookies), 200)
	if invoices[0].Balance != inv.Balance-20 {
		t.Fatal("credit not reflected")
	}
}
func TestWorkflowsLeadConversionTemplatesAndCurrencies(t *testing.T) {
	setupStore(t)
	h := featureMux()
	cookies := demoCookies(t, h)
	lead := leads[0]
	before := len(contacts)
	assertStatus(t, featureRequest(h, "POST", "/api/workflows/lead-convert", map[string]string{"id": lead.ID}, cookies), 200)
	assertStatus(t, featureRequest(h, "POST", "/api/workflows/lead-convert", map[string]string{"id": lead.ID}, cookies), 200)
	if len(contacts) > before+1 {
		t.Fatal("conversion not idempotent")
	}
	d := deals[0]
	assertStatus(t, featureRequest(h, "POST", "/api/workflows/deal", map[string]any{"id": d.ID, "currency": "EUR", "probability": 65, "lineItems": []LineItem{{"Service", 2, 100}}}, cookies), 200)
	if deals[0].Value != 200 || deals[0].Currency != "EUR" {
		t.Fatal("line item / currency missing")
	}
	r := featureRequest(h, "GET", "/api/reports", nil, cookies)
	assertStatus(t, r, 200)
	if !strings.Contains(r.Body.String(), "EUR") {
		t.Fatal("currency totals missing")
	}
	assertStatus(t, featureRequest(h, "POST", "/api/workflows/student-case", map[string]string{"id": students[0].ID}, cookies), 201)
	if cases[len(cases)-1].StudentID != students[0].ID {
		t.Fatal("student not linked")
	}
}
func TestSignedGatewayCallbackIsIdempotent(t *testing.T) {
	setupStore(t)
	t.Setenv("STRIPE_WEBHOOK_SECRET", "fixture-secret")
	h := featureMux()
	inv := invoices[0]
	body := []byte(mustJSON(map[string]any{"id": "evt_fixture", "type": "checkout.session.completed", "data": map[string]any{"object": map[string]any{"id": "cs_fixture", "amount_total": 10000, "currency": "usd", "payment_status": "paid", "payment_intent": "pi_fixture", "metadata": map[string]string{"invoice_id": inv.ID, "org_id": inv.OrgID}}}}))
	stamp := fmt.Sprint(time.Now().Unix())
	mac := hmac.New(sha256.New, []byte("fixture-secret"))
	mac.Write([]byte(stamp + "."))
	mac.Write(body)
	header := "t=" + stamp + ",v1=" + hex.EncodeToString(mac.Sum(nil))
	for i := 0; i < 2; i++ {
		r := httptest.NewRequest("POST", "/api/payments/stripe/webhook", bytes.NewReader(body))
		r.Header.Set("Stripe-Signature", header)
		w := httptest.NewRecorder()
		serveFixtureRequest(h, w, r)
		assertStatus(t, w, 200)
	}
	if invoices[0].AmountReceived != inv.AmountReceived+100 {
		t.Fatal("duplicate gateway payment")
	}
	if stripeSignatureValid("t=1,v1=invalid", body) {
		t.Fatal("invalid signature accepted")
	}
}
func TestCountryPhoneAndURLValidation(t *testing.T) {
	for _, input := range []string{"ca", "CA", "Canada"} {
		if value, ok := normalizedCountry(input); !ok || value != "CA" {
			t.Fatal(input)
		}
	}
	if _, ok := normalizedCountry("not a country"); ok {
		t.Fatal("invalid country")
	}
	if validPhone("12") || !validPhone("+63 917 123 4567") {
		t.Fatal("phone normalization")
	}
	if publicURL("http://127.0.0.1") || allowedPush("https://evil.example/push") {
		t.Fatal("unsafe endpoint")
	}
}
func TestEncryptedSecretsNotPublic(t *testing.T) {
	setupStore(t)
	secret, err := encryptSecret("SMTP-private-password")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(secret, "SMTP-private-password") {
		t.Fatal("plaintext secret")
	}
	plain, err := decryptSecret(secret)
	if err != nil || plain != "SMTP-private-password" {
		t.Fatal("encryption roundtrip")
	}
	u, _ := findUserByEmail("demo@northwind.dev")
	hash, _ := bcrypt.GenerateFromPassword([]byte("FixturePassword!"), bcrypt.MinCost)
	u.Password = string(hash)
	u.TOTPSecret = secret
	if strings.Contains(mustJSON(u), secret) || strings.Contains(mustJSON(u), u.Password) {
		t.Fatal("user secret exposed")
	}
	path := filepath.Join(filepath.Dir(dataPath), "uploads")
	if _, err = os.Stat(path); err == nil {
		t.Fatal("unexpected test uploads")
	}
}

func TestCaseTemplatesVerificationAndTaskRecurrence(t *testing.T) {
	setupStore(t)
	h := featureMux()
	cookies := demoCookies(t, h)
	w := featureRequest(h, "POST", "/api/workflows/student-case", map[string]string{"id": students[0].ID}, cookies)
	assertStatus(t, w, 201)
	var result struct{ Case Case }
	json.Unmarshal(w.Body.Bytes(), &result)
	id := result.Case.ID
	w = featureRequest(h, "POST", "/api/workflows/case-template", map[string]string{"name": "Student visa", "documents": "Passport\nAdmission letter"}, cookies)
	assertStatus(t, w, 201)
	var template map[string]string
	json.Unmarshal(w.Body.Bytes(), &template)
	assertStatus(t, featureRequest(h, "POST", "/api/workflows/case", map[string]any{"id": id, "templateId": template["id"], "slaDays": 14, "autoAdvance": true}, cookies), 200)
	assertStatus(t, featureRequest(h, "PATCH", "/api/cases/"+id+"/stage", map[string]string{"currentStage": "Documents Pending"}, cookies), 200)
	docs := []Document{}
	for _, d := range documents {
		if d.CaseID == id {
			docs = append(docs, d)
		}
	}
	if len(docs) != 2 {
		t.Fatal("template checklist missing")
	}
	assertStatus(t, featureRequest(h, "PATCH", "/api/documents/"+docs[0].ID+"/status", map[string]string{"status": "Verified"}, cookies), 409)
	for _, d := range docs {
		assertStatus(t, multipartRequest(h, "/api/documents/"+d.ID+"/files", "file", "file.pdf", []byte("%PDF-1.4 fixture"), cookies, nil), 201)
		assertStatus(t, featureRequest(h, "PATCH", "/api/documents/"+d.ID+"/status", map[string]string{"status": "Verified"}, cookies), 200)
	}
	mu.RLock()
	c, _ := findCaseByID(id)
	mu.RUnlock()
	if c.CurrentStage != "Ready for Submission" || c.NextDeadline == "" {
		t.Fatal("case did not auto advance with SLA", c)
	}
	mu.Lock()
	tasks[0].Status = "done"
	tasks[0].Recurrence = "weekly"
	tasks[0].DueDate = "2026-10-06"
	mu.Unlock()
	before := len(tasks)
	commitFixtureEdits()
	generateReminders()
	reloadFixtureAssertions()
	commitFixtureEdits()
	generateReminders()
	reloadFixtureAssertions()
	if len(tasks) != before+1 || tasks[len(tasks)-1].DueDate != "2026-10-13" || tasks[len(tasks)-1].Status != "todo" {
		t.Fatal("recurrence missing or duplicated")
	}
}

func TestHiddenPaymentStillCountsInInvoiceBalance(t *testing.T) {
	setupStore(t)
	h := featureMux()
	admin := demoCookies(t, h)
	assertStatus(t, featureRequest(h, "POST", "/api/admin/users", map[string]string{"name": "Member", "email": "ledger-member@example.com", "password": "LedgerMemberPassword!", "role": "member"}, admin), 201)
	u, _ := findUserByEmail("ledger-member@example.com")
	mu.Lock()
	invoices[0].OwnerID = u.ID
	payments[0].Visibility = "private"
	original := invoices[0].AmountReceived
	invoiceID := invoices[0].ID
	mu.Unlock()
	if err := saveStore(); err != nil {
		t.Fatal(err)
	}
	w := featureRequest(h, "POST", "/api/login", map[string]string{"email": u.Email, "password": "LedgerMemberPassword!"}, nil)
	assertStatus(t, w, 200)
	cookies := w.Result().Cookies()
	w = featureRequest(h, "POST", "/api/payments", map[string]any{"invoiceId": invoiceID, "amount": 100, "paidOn": "2026-10-06", "method": "Bank"}, cookies)
	assertStatus(t, w, 201)
	if invoices[0].AmountReceived != original+100 {
		t.Fatal("hidden receipt was lost from the balance", invoices[0].AmountReceived)
	}
}
