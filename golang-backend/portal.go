package main

import (
	"context"
	"net/http"
	"strings"
	"time"
)

type portalGrant struct{ ID, OrgID, StudentID, ContactID, Email string }

func registerPortalRoutes(m *http.ServeMux) {
	m.HandleFunc("POST /api/portal/invitations", authMiddleware(handlePortalInvite))
	m.HandleFunc("GET /api/portal/invitations", authMiddleware(handlePortalInvites))
	m.HandleFunc("DELETE /api/portal/invitations/{grant}", authMiddleware(handlePortalRevoke))
	m.HandleFunc("POST /api/portal/session", handlePortalExchange)
	m.HandleFunc("POST /api/portal/logout", handlePortalLogout)
	m.HandleFunc("GET /api/portal/me", portalMiddleware(handlePortalMe))
	m.HandleFunc("POST /api/portal/messages", portalMiddleware(handlePortalMessage))
	m.HandleFunc("POST /api/portal/documents/{id}/files", portalMiddleware(handlePortalUpload))
	m.HandleFunc("GET /api/portal/documents/{id}/files/{version}", portalMiddleware(handlePortalDownload))
	m.HandleFunc("GET /api/portal/invoices/{id}/pdf", portalMiddleware(handlePortalInvoice))
	m.HandleFunc("GET /portal", func(w http.ResponseWriter, r *http.Request) {
		serveFileIfExists(w, r, findProjectRoot()+"/public/portal.html", "text/html; charset=utf-8")
	})
}
func (state *runtimeStore) handlePortalInvite(w http.ResponseWriter, r *http.Request) {
	if !isManager(currentUser(r)) {
		writeError(w, 403, "Manager access is required")
		return
	}
	var req struct{ StudentID, ContactID, Email string }
	if !decodeRequest(w, r, &req) {
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if !validEmail(req.Email) {
		writeError(w, 400, "Enter the client's email")
		return
	}
	s := state.snapshot()
	allowed := false
	if req.StudentID != "" {
		for _, student := range s.Students {
			if student.ID == req.StudentID {
				allowed = true
			}
		}
	}
	if req.StudentID == "" {
		for _, contact := range s.Contacts {
			if contact.ID == req.ContactID {
				allowed = true
			}
		}
	}
	if !allowed {
		writeError(w, 404, "Student or contact not found")
		return
	}
	if req.StudentID != "" {
		linked := ""
		for _, c := range s.Cases {
			if c.StudentID == req.StudentID {
				if linked != "" && linked != c.ClientID {
					writeError(w, 409, "Student cases have different clients; resolve them before inviting")
					return
				}
				linked = c.ClientID
			}
		}
		if linked == "" {
			writeError(w, 409, "Create a linked student case before inviting this student")
			return
		}
		req.ContactID = linked
	}
	token, id := randomSecret(), newID("portal")
	if _, err := storeDB(r).Exec("INSERT INTO portal_access(id,org_id,student_id,contact_id,email,token_hash,created_at,expires_at,revoked_at) VALUES(?,?,?,?,?,?,?,?,NULL)", id, currentUser(r).OrgID, req.StudentID, req.ContactID, req.Email, hashSecret(token), utcNow(), time.Now().Add(72*time.Hour).UTC().Format(time.RFC3339)); err != nil {
		writeError(w, 500, "Could not create portal invitation")
		return
	}
	link := strings.TrimRight(appURL(), "/") + "/portal#invite=" + token
	writeJSON(w, 201, map[string]string{"id": id, "url": link, "message": "Share this private, single-use link with the client. It expires in 72 hours."})
}

func handlePortalInvite(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handlePortalInvite(w, r)
}
func handlePortalInvites(w http.ResponseWriter, r *http.Request) {
	if !isManager(currentUser(r)) {
		writeError(w, 403, "Manager access is required")
		return
	}
	rows, err := queryObjects(r, "SELECT id,student_id AS studentId,contact_id AS contactId,email,created_at AS createdAt,expires_at AS expiresAt,revoked_at AS revokedAt FROM portal_access WHERE org_id=? ORDER BY created_at DESC LIMIT 100", currentUser(r).OrgID)
	if err != nil {
		writeError(w, 500, "Could not load invitations")
		return
	}
	writeJSON(w, 200, map[string]any{"invitations": rows})
}
func handlePortalRevoke(w http.ResponseWriter, r *http.Request) {
	if !isManager(currentUser(r)) {
		writeError(w, 403, "Manager access is required")
		return
	}
	result, err := storeDB(r).Exec("UPDATE portal_access SET revoked_at=? WHERE id=? AND org_id=?", utcNow(), r.PathValue("grant"), currentUser(r).OrgID)
	if err != nil {
		writeError(w, 500, "Could not revoke access")
		return
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		writeError(w, 404, "Access not found")
		return
	}
	writeJSON(w, 200, statusResponse{"revoked"})
}
func handlePortalExchange(w http.ResponseWriter, r *http.Request) {
	var req struct{ Token string }
	if !decodeRequest(w, r, &req) {
		return
	}
	if len(req.Token) < 32 || len(req.Token) > 200 {
		writeError(w, 401, "Invalid or expired invitation")
		return
	}
	var id string
	if storeDB(r).QueryRow("SELECT id FROM portal_access WHERE token_hash=? AND activated_at IS NULL AND revoked_at IS NULL AND expires_at>?", hashSecret(req.Token), utcNow()).Scan(&id) != nil {
		writeError(w, 401, "Invalid or expired invitation")
		return
	}
	token := randomSecret()
	expiry := time.Now().Add(7 * 24 * time.Hour)
	if _, err := storeDB(r).Exec("UPDATE portal_access SET token_hash=?,expires_at=?,activated_at=? WHERE id=?", hashSecret(token), expiry.UTC().Format(time.RFC3339), utcNow(), id); err != nil {
		writeError(w, 500, "Could not sign in")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "ecc_portal", Value: token, Path: "/api/portal", HttpOnly: true, Secure: secureCookie(r), SameSite: http.SameSiteStrictMode, MaxAge: 7 * 24 * 3600})
	writeJSON(w, 200, statusResponse{"ok"})
}

const portalKey contextKey = "portalGrant"

func portalMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("ecc_portal")
		if err != nil {
			writeError(w, 401, "Open your private invitation to sign in")
			return
		}
		var g portalGrant
		err = storeDB(r).QueryRow("SELECT id,org_id,student_id,contact_id,email FROM portal_access WHERE token_hash=? AND activated_at IS NOT NULL AND revoked_at IS NULL AND expires_at>?", hashSecret(cookie.Value), utcNow()).Scan(&g.ID, &g.OrgID, &g.StudentID, &g.ContactID, &g.Email)
		if err != nil {
			writeError(w, 401, "Your portal access expired or was revoked")
			return
		}
		if meta := metadata(r); meta != nil {
			meta.Actor = "portal:" + g.Email
			meta.OrgID = g.OrgID
		}
		next(w, r.WithContext(context.WithValue(r.Context(), portalKey, g)))
	}
}
func portalIdentity(r *http.Request) portalGrant {
	g, _ := r.Context().Value(portalKey).(portalGrant)
	return g
}
func (state *runtimeStore) portalCases(g portalGrant) map[string]Case {
	out := map[string]Case{}
	for _, c := range state.snapshot().Cases {
		if c.OrgID == g.OrgID && c.ClientID == g.ContactID && ((g.StudentID != "" && c.StudentID == g.StudentID) || (g.StudentID == "")) {
			out[c.ID] = c
		}
	}
	return out
}

func portalCases(g portalGrant) map[string]Case {
	state, release := runtimeForReadCall(nil)
	defer release()
	return state.portalCases(g)
}
func (state *runtimeStore) portalDocument(r *http.Request) (Document, bool) {
	g := portalIdentity(r)
	allowed := state.portalCases(g)
	for _, d := range state.snapshot().Documents {
		if d.ID == r.PathValue("id") && d.OrgID == g.OrgID {
			_, ok := allowed[d.CaseID]
			return d, ok
		}
	}
	return Document{}, false
}

func portalDocument(r *http.Request) (Document, bool) {
	state, release := runtimeForReadCall(r)
	defer release()
	return state.portalDocument(r)
}
func (state *runtimeStore) portalInvoice(r *http.Request) (Invoice, bool) {
	g := portalIdentity(r)
	allowed := state.portalCases(g)
	for _, i := range state.snapshot().Invoices {
		if i.ID == r.PathValue("id") && i.OrgID == g.OrgID && i.ClientID == g.ContactID {
			_, ok := allowed[i.CaseID]
			return i, ok || (i.CaseID == "" && g.StudentID == "")
		}
	}
	return Invoice{}, false
}

func portalInvoice(r *http.Request) (Invoice, bool) {
	state, release := runtimeForReadCall(r)
	defer release()
	return state.portalInvoice(r)
}
func (state *runtimeStore) handlePortalMe(w http.ResponseWriter, r *http.Request) {
	g := portalIdentity(r)
	s := state.snapshot()
	allowed := state.portalCases(g)
	cs, ds, is := []map[string]any{}, []map[string]any{}, []map[string]any{}
	for _, c := range allowed {
		cs = append(cs, map[string]any{"id": c.ID, "caseNumber": c.CaseNumber, "stage": c.CurrentStage, "nextAction": c.NextAction, "deadline": c.NextDeadline, "destination": c.DestinationCountry})
	}
	for _, d := range s.Documents {
		if _, ok := allowed[d.CaseID]; ok && d.OrgID == g.OrgID {
			ds = append(ds, map[string]any{"id": d.ID, "name": d.DocName, "caseNumber": d.CaseNumber, "required": d.Required, "status": d.Status, "latestVersion": d.LatestVersion, "rejectionReason": d.RejectionReason})
		}
	}
	for _, i := range s.Invoices {
		_, linked := allowed[i.CaseID]
		if i.OrgID == g.OrgID && i.ClientID == g.ContactID && (linked || (i.CaseID == "" && g.StudentID == "")) {
			is = append(is, map[string]any{"id": i.ID, "number": i.InvoiceNumber, "balance": i.Balance, "currency": currencyOf(i.Currency), "dueDate": i.DueDate})
		}
	}
	var student any
	for _, st := range s.Students {
		if st.ID == g.StudentID && st.OrgID == g.OrgID {
			student = map[string]any{"name": st.Name, "program": st.Program, "applicationStage": st.ApplicationStage, "visaStatus": st.VisaStatus, "preDeparture": st.PreDeparture}
		}
	}
	messages := []map[string]any{}
	rows, err := queryObjects(r, "SELECT id,case_id AS caseId,body,created_at AS createdAt FROM conversations WHERE org_id=? AND contact_id=? AND channel='portal' ORDER BY created_at DESC LIMIT 50", g.OrgID, g.ContactID)
	if err != nil {
		writeError(w, 500, "Could not load portal messages")
		return
	}
	for _, m := range rows {
		if _, ok := allowed[fmtString(m["caseId"])]; ok {
			messages = append(messages, m)
		}
	}
	writeJSON(w, 200, map[string]any{"email": g.Email, "student": student, "cases": cs, "documents": ds, "invoices": is, "messages": messages})
}

func handlePortalMe(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handlePortalMe(w, r)
}
func fmtString(v any) string { s, _ := v.(string); return s }
func (state *runtimeStore) handlePortalMessage(w http.ResponseWriter, r *http.Request) {
	var req struct{ CaseID, Body string }
	if !decodeRequest(w, r, &req) {
		return
	}
	g := portalIdentity(r)
	c, ok := state.portalCases(g)[req.CaseID]
	if !ok {
		writeError(w, 404, "Case not found")
		return
	}
	req.Body = strings.TrimSpace(req.Body)
	if req.Body == "" || len(req.Body) > 10000 {
		writeError(w, 400, "Enter a message up to 10,000 characters")
		return
	}
	if _, err := storeDB(r).Exec("INSERT INTO conversations VALUES(?,?,?,?,?,?,?,?,?,?,?)", newID("message"), g.OrgID, g.ContactID, c.ID, "portal", "inbound", newID("portalmsg"), "Client response", req.Body, "received", utcNow()); err != nil {
		writeError(w, 500, "Could not save message")
		return
	}
	if err := portalNotify(r, c.RecordScope, "Client portal response", req.Body, "/cases/"+c.ID); err != nil {
		writeError(w, 500, "Could not notify your adviser")
		return
	}
	state.mu.Lock()
	state.activities = append(state.activities, Activity{RecordScope: c.RecordScope, ID: newID("a"), ContactID: g.ContactID, Kind: "note", Title: "Portal response · " + c.CaseNumber, Body: req.Body, OccurredAt: utcNow(), CreatedAt: utcNow(), CreatedBy: g.Email})
	state.mu.Unlock()
	writeJSON(w, 201, statusResponse{"received"})
}

func handlePortalMessage(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handlePortalMessage(w, r)
}
func portalStaffRequest(r *http.Request, scope RecordScope) *http.Request {
	u := User{ID: scope.OwnerID, OrgID: scope.OrgID, TeamID: scope.TeamID, Role: "member", Name: portalIdentity(r).Email}
	return r.WithContext(context.WithValue(r.Context(), requestUserKey, u))
}
func (state *runtimeStore) handlePortalUpload(w http.ResponseWriter, r *http.Request) {
	d, ok := state.portalDocument(r)
	if !ok {
		writeError(w, 404, "Document not found")
		return
	}
	response := &bufferedResponse{header: make(http.Header)}
	state.handleFileUpload(response, portalStaffRequest(r, d.RecordScope))
	if response.status >= 200 && response.status < 300 {
		if err := portalNotify(r, d.RecordScope, "Client document uploaded", d.DocName, "/cases/"+d.CaseID); err != nil {
			writeError(w, 500, "Could not notify your adviser")
			return
		}
		writeJSON(w, response.status, statusResponse{"uploaded"})
		return
	}
	for key, values := range response.header {
		w.Header()[key] = values
	}
	w.WriteHeader(response.status)
	w.Write(response.body.Bytes())
}

func handlePortalUpload(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handlePortalUpload(w, r)
}
func (state *runtimeStore) handlePortalDownload(w http.ResponseWriter, r *http.Request) {
	d, ok := state.portalDocument(r)
	if !ok {
		writeError(w, 404, "Document not found")
		return
	}
	state.handleFileDownload(w, portalStaffRequest(r, d.RecordScope))
}

func handlePortalDownload(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handlePortalDownload(w, r)
}
func (state *runtimeStore) handlePortalInvoice(w http.ResponseWriter, r *http.Request) {
	i, ok := state.portalInvoice(r)
	if !ok {
		writeError(w, 404, "Invoice not found")
		return
	}
	i.Notes = ""
	data, err := invoicePDF(i)
	if err != nil {
		writeError(w, 500, "Could not create invoice")
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="invoice.pdf"`)
	w.Write(data)
}

func handlePortalInvoice(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handlePortalInvoice(w, r)
}
func handlePortalLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("ecc_portal"); err == nil {
		if _, err = storeDB(r).Exec("UPDATE portal_access SET revoked_at=? WHERE token_hash=?", utcNow(), hashSecret(cookie.Value)); err != nil {
			writeError(w, 500, "Could not sign out")
			return
		}
	}
	http.SetCookie(w, &http.Cookie{Name: "ecc_portal", Path: "/api/portal", HttpOnly: true, Secure: secureCookie(r), SameSite: http.SameSiteStrictMode, MaxAge: -1})
	writeJSON(w, 200, statusResponse{"ok"})
}

func portalNotify(r *http.Request, scope RecordScope, title, body, path string) error {
	if scope.OwnerID == "" {
		return nil
	}
	_, err := storeDB(r).Exec("INSERT INTO notifications VALUES(?,?,?,?,?,?,?,?,?,NULL)", newID("notice"), scope.OrgID, scope.OwnerID, "portal", title, body, path, newID("portal_event"), utcNow())
	return err
}
