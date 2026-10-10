package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func registerConversationRoutes(m *http.ServeMux) {
	m.HandleFunc("GET /api/contacts/{id}/conversations", authMiddleware(handleConversations))
	m.HandleFunc("POST /api/contacts/{id}/whatsapp", authMiddleware(handleWhatsAppSend))
	m.HandleFunc("POST /api/inbound/email", handleInboundEmail)
	m.HandleFunc("GET /api/inbound/whatsapp", handleWhatsAppChallenge)
	m.HandleFunc("POST /api/inbound/whatsapp", handleWhatsAppWebhook)
	m.HandleFunc("GET /api/inbound/unmatched", authMiddleware(handleUnmatched))
	m.HandleFunc("POST /api/inbound/unmatched/{message}/link", authMiddleware(handleLinkMessage))
	m.HandleFunc("GET /api/mail/sync-status", authMiddleware(handleMailboxStatus))
}
func (state *runtimeStore) conversationContact(r *http.Request, id string) (Contact, bool) {
	for _, c := range state.snapshot().Contacts {
		if c.ID == id && canRead(c.RecordScope, currentUser(r)) {
			return c, true
		}
	}
	return Contact{}, false
}

func conversationContact(r *http.Request, id string) (Contact, bool) {
	state, release := runtimeForReadCall(r)
	defer release()
	return state.conversationContact(r, id)
}
func (state *runtimeStore) handleConversations(w http.ResponseWriter, r *http.Request) {
	if _, ok := state.conversationContact(r, r.PathValue("id")); !ok {
		writeError(w, 404, "Contact not found")
		return
	}
	limit, offset := parseLimitOffset(r, 25, 0)
	rows, err := queryObjects(r, "SELECT id,channel,direction,subject,body,status,case_id AS caseId,created_at AS createdAt FROM conversations WHERE org_id=? AND contact_id=? ORDER BY created_at DESC LIMIT ? OFFSET ?", currentUser(r).OrgID, r.PathValue("id"), limit, offset)
	if err != nil {
		writeError(w, 500, "Could not load conversation")
		return
	}
	safe := []map[string]any{}
	for _, m := range rows {
		caseID := fmtString(m["caseId"])
		if caseID == "" || visibleRecordID(state.snapshot(), caseID) {
			safe = append(safe, m)
		}
	}
	writeJSON(w, 200, map[string]any{"messages": safe})
}

func handleConversations(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleConversations(w, r)
}

type inboundMessage struct {
	ID         string `json:"id"`
	OrgID      string `json:"orgId"`
	From       string `json:"from"`
	Subject    string `json:"subject"`
	Body       string `json:"body"`
	CaseNumber string `json:"caseNumber"`
	OccurredAt string `json:"occurredAt"`
}

func (state *runtimeStore) findInboundContact(org, channel, sender string) (Contact, bool) {
	matches := []Contact{}
	for _, c := range state.snapshot().Contacts {
		if c.OrgID != org {
			continue
		}
		if (channel == "email" && strings.EqualFold(strings.TrimSpace(c.Email), strings.TrimSpace(sender))) || (channel == "whatsapp" && normalizePhone(c.Phone) == normalizePhone("+"+strings.TrimPrefix(sender, "+"))) {
			matches = append(matches, c)
		}
	}
	if len(matches) != 1 {
		return Contact{}, false
	}
	return matches[0], true
}

func findInboundContact(org, channel, sender string) (Contact, bool) {
	state, release := runtimeForReadCall(nil)
	defer release()
	return state.findInboundContact(org, channel, sender)
}
func (state *runtimeStore) attachInbound(r *http.Request, channel string, m inboundMessage) error {
	if m.ID == "" || len(m.ID) > 500 || m.OrgID == "" || len(m.Subject) > 200 || len(m.Body) > 100000 || len(m.From) > 300 {
		return fmt.Errorf("invalid inbound message")
	}
	var count int
	if err := storeDB(r).QueryRow("SELECT count(*) FROM organizations WHERE id=?", m.OrgID).Scan(&count); err != nil || count != 1 {
		return fmt.Errorf("unknown organization")
	}
	at := utcNow()
	if m.OccurredAt != "" {
		parsed, e := time.Parse(time.RFC3339, m.OccurredAt)
		if e != nil || time.Until(parsed) > 5*time.Minute {
			return fmt.Errorf("invalid message timestamp")
		}
		at = parsed.UTC().Format(time.RFC3339)
	}
	c, matched := state.findInboundContact(m.OrgID, channel, m.From)
	caseID := ""
	scope := c.RecordScope
	if matched {
		matchingCases := []Case{}
		for _, candidate := range state.snapshot().Cases {
			if candidate.OrgID == m.OrgID && candidate.ClientID == c.ID && ((m.CaseNumber != "" && candidate.CaseNumber == m.CaseNumber) || (m.CaseNumber == "" && subjectCaseReference(m.Subject, candidate.CaseNumber))) {
				matchingCases = append(matchingCases, candidate)
			}
		}
		if len(matchingCases) == 1 {
			caseID = matchingCases[0].ID
			scope = matchingCases[0].RecordScope
		}
	}

	subject := m.Subject
	if !matched {
		subject = "From " + m.From + " · " + m.Subject
		if len(subject) > 500 {
			subject = subject[:500]
		}
	}
	result, err := storeDB(r).Exec("INSERT OR IGNORE INTO conversations VALUES(?,?,?,?,?,?,?,?,?,?,?)", newID("message"), m.OrgID, c.ID, caseID, channel, "inbound", m.ID, subject, m.Body, "received", at)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return nil
	}
	if matched {
		activityKind := channel
		if activityKind != "whatsapp" {
			activityKind = "email"
		}
		state.mu.Lock()
		state.activities = append(state.activities, Activity{RecordScope: scope, ID: newID("a"), ContactID: c.ID, Kind: activityKind, Title: "Incoming " + channel + " · " + m.Subject, Body: m.Body, CreatedBy: m.From, OccurredAt: at, CreatedAt: utcNow()})
		state.mu.Unlock()
	}
	if meta := metadata(r); meta != nil {
		meta.OrgID = m.OrgID
		meta.Actor = "inbound:" + channel
	}
	return nil
}

func attachInbound(r *http.Request, channel string, m inboundMessage) error {
	state, release := runtimeForCall(r)
	defer release()
	return state.attachInbound(r, channel, m)
}
func signedInbound(secret, signature string, body []byte) bool {
	if secret == "" {
		return false
	}
	want := hmac.New(sha256.New, []byte(secret))
	want.Write(body)
	got, err := hex.DecodeString(strings.TrimPrefix(signature, "sha256="))
	return err == nil && hmac.Equal(got, want.Sum(nil))
}
func (state *runtimeStore) handleInboundEmail(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 2<<20))
	if err != nil {
		writeError(w, 400, "Message too large")
		return
	}
	stamp := r.Header.Get("X-ECC-Timestamp")
	seconds, err := strconv.ParseInt(stamp, 10, 64)
	if err != nil || time.Since(time.Unix(seconds, 0)) > 5*time.Minute || time.Until(time.Unix(seconds, 0)) > time.Minute || !signedInbound(os.Getenv("INBOUND_EMAIL_SECRET"), r.Header.Get("X-ECC-Signature"), append([]byte(stamp+"."), body...)) {
		writeError(w, 401, "Invalid inbound signature")
		return
	}
	var m inboundMessage
	if json.Unmarshal(body, &m) != nil || !validEmail(m.From) {
		writeError(w, 400, "Invalid email payload")
		return
	}
	if err = state.attachInbound(r, "email", m); err != nil {
		writeError(w, 400, "Could not accept email")
		return
	}
	writeJSON(w, 202, statusResponse{"received"})
}

func handleInboundEmail(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleInboundEmail(w, r)
}
func handleUnmatched(w http.ResponseWriter, r *http.Request) {
	if !isManager(currentUser(r)) {
		writeError(w, 403, "Manager access is required")
		return
	}
	rows, err := queryObjects(r, "SELECT id,channel,subject,body,created_at AS createdAt FROM conversations WHERE org_id=? AND contact_id='' ORDER BY created_at DESC LIMIT 100", currentUser(r).OrgID)
	if err != nil {
		writeError(w, 500, "Could not load unmatched messages")
		return
	}
	writeJSON(w, 200, map[string]any{"messages": rows})
}
func (state *runtimeStore) handleLinkMessage(w http.ResponseWriter, r *http.Request) {
	if !isManager(currentUser(r)) {
		writeError(w, 403, "Manager access is required")
		return
	}
	var req struct{ ContactID string }
	if !decodeRequest(w, r, &req) {
		return
	}
	c, ok := state.conversationContact(r, req.ContactID)
	if !ok {
		writeError(w, 404, "Contact not found")
		return
	}
	var subject, body, channel string
	if storeDB(r).QueryRow("SELECT subject,body,channel FROM conversations WHERE id=? AND org_id=? AND contact_id=''", r.PathValue("message"), currentUser(r).OrgID).Scan(&subject, &body, &channel) != nil {
		writeError(w, 404, "Unmatched message not found")
		return
	}
	if _, err := storeDB(r).Exec("UPDATE conversations SET contact_id=? WHERE id=?", c.ID, r.PathValue("message")); err != nil {
		writeError(w, 500, "Could not attach message")
		return
	}
	activityKind := channel
	if activityKind != "whatsapp" {
		activityKind = "email"
	}
	state.mu.Lock()
	state.activities = append(state.activities, Activity{RecordScope: c.RecordScope, ID: newID("a"), ContactID: c.ID, Kind: activityKind, Title: "Incoming " + channel + " · " + subject, Body: body, CreatedBy: currentUser(r).Name, CreatedAt: utcNow(), OccurredAt: utcNow()})
	state.mu.Unlock()
	writeJSON(w, 200, statusResponse{"linked"})
}

func handleLinkMessage(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleLinkMessage(w, r)
}
func handleWhatsAppChallenge(w http.ResponseWriter, r *http.Request) {
	token := os.Getenv("WHATSAPP_VERIFY_TOKEN")
	if token == "" || r.URL.Query().Get("hub.mode") != "subscribe" || !hmac.Equal([]byte(r.URL.Query().Get("hub.verify_token")), []byte(token)) {
		writeError(w, 403, "Invalid verification token")
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(r.URL.Query().Get("hub.challenge")))
}
func (state *runtimeStore) handleWhatsAppWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 2<<20))
	if err != nil {
		writeError(w, 400, "Webhook too large")
		return
	}
	if !signedInbound(os.Getenv("WHATSAPP_APP_SECRET"), r.Header.Get("X-Hub-Signature-256"), body) {
		writeError(w, 401, "Invalid webhook signature")
		return
	}
	var event struct {
		Entry []struct {
			Changes []struct {
				Value struct {
					Metadata struct {
						PhoneNumberID string `json:"phone_number_id"`
					} `json:"metadata"`
					Messages []struct {
						ID, From, Type, Timestamp string
						Text                      struct{ Body string }
					} `json:"messages"`
					Statuses []struct{ ID, Status string } `json:"statuses"`
				} `json:"value"`
			} `json:"changes"`
		} `json:"entry"`
	}
	if json.Unmarshal(body, &event) != nil {
		writeError(w, 400, "Invalid webhook")
		return
	}
	org := os.Getenv("WHATSAPP_ORG_ID")
	if org == "" {
		writeError(w, 503, "WhatsApp workspace is not configured")
		return
	}
	for _, entry := range event.Entry {
		for _, change := range entry.Changes {
			v := change.Value
			if v.Metadata.PhoneNumberID != os.Getenv("WHATSAPP_PHONE_NUMBER_ID") {
				continue
			}
			for _, m := range v.Messages {
				if m.Type != "text" {
					continue
				}
				if err = state.attachInbound(r, "whatsapp", inboundMessage{ID: m.ID, OrgID: org, From: m.From, Subject: "WhatsApp reply", Body: m.Text.Body, OccurredAt: whatsAppTime(m.Timestamp)}); err != nil {
					writeError(w, 500, "Could not save reply")
					return
				}
			}
			for _, s := range v.Statuses {
				if s.Status == "sent" || s.Status == "delivered" || s.Status == "read" || s.Status == "failed" {
					if _, err = storeDB(r).Exec("UPDATE conversations SET status=? WHERE org_id=? AND channel='whatsapp' AND direction='outbound' AND provider_id=? AND (status NOT IN ('read','delivered') OR ?='read')", s.Status, org, s.ID, s.Status); err != nil {
						writeError(w, 500, "Could not save delivery status")
						return
					}
				}
			}
		}
	}
	writeJSON(w, 200, statusResponse{"ok"})
}

func handleWhatsAppWebhook(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleWhatsAppWebhook(w, r)
}
func (state *runtimeStore) handleWhatsAppSend(w http.ResponseWriter, r *http.Request) {
	c, ok := state.conversationContact(r, r.PathValue("id"))
	if !ok {
		writeError(w, 404, "Contact not found")
		return
	}
	if !canWrite(c.RecordScope, currentUser(r)) {
		writeError(w, 403, "You cannot message this contact")
		return
	}
	if c.OrgID != os.Getenv("WHATSAPP_ORG_ID") || os.Getenv("WHATSAPP_ACCESS_TOKEN") == "" || os.Getenv("WHATSAPP_API_VERSION") == "" {
		writeError(w, 503, "WhatsApp is not configured for this workspace")
		return
	}
	var req struct{ Body string }
	if !decodeRequest(w, r, &req) {
		return
	}
	req.Body = strings.TrimSpace(req.Body)
	if req.Body == "" || len(req.Body) > 4096 || normalizePhone(c.Phone) == "" {
		writeError(w, 400, "Enter a message up to 4,096 characters and a valid contact phone")
		return
	}
	var recent int
	err := storeDB(r).QueryRow("SELECT count(*) FROM conversations WHERE org_id=? AND contact_id=? AND channel='whatsapp' AND direction='inbound' AND created_at>?", c.OrgID, c.ID, time.Now().Add(-24*time.Hour).UTC().Format(time.RFC3339)).Scan(&recent)
	if err != nil || recent == 0 {
		writeError(w, 409, "Free-form WhatsApp replies require an incoming message within the last 24 hours")
		return
	}
	id := newID("message")
	if _, err = storeDB(r).Exec("INSERT INTO conversations VALUES(?,?,?,?,?,?,?,?,?,?,?)", id, c.OrgID, c.ID, "", "whatsapp", "outbound", id, "WhatsApp reply", req.Body, "queued", utcNow()); err != nil {
		writeError(w, 500, "Could not queue message")
		return
	}
	writeJSON(w, 202, map[string]string{"id": id, "status": "queued"})
}

func handleWhatsAppSend(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleWhatsAppSend(w, r)
}
func processWhatsApp(ctx context.Context) {
	org := os.Getenv("WHATSAPP_ORG_ID")
	if org == "" || os.Getenv("WHATSAPP_ACCESS_TOKEN") == "" {
		return
	}
	var id, contact, body string
	if database.QueryRow("SELECT id,contact_id,body FROM conversations WHERE org_id=? AND channel='whatsapp' AND direction='outbound' AND status='queued' ORDER BY created_at LIMIT 1", org).Scan(&id, &contact, &body) != nil {
		return
	}
	var payload string
	if database.QueryRow("SELECT data FROM contacts WHERE id=? AND json_extract(data,'$.orgId')=?", contact, org).Scan(&payload) != nil {
		database.Exec("UPDATE conversations SET status='failed' WHERE id=?", id)
		return
	}
	var c Contact
	json.Unmarshal([]byte(payload), &c)
	b, _ := json.Marshal(map[string]any{"messaging_product": "whatsapp", "to": strings.TrimPrefix(normalizePhone(c.Phone), "+"), "type": "text", "text": map[string]string{"body": body}})
	version, phone := os.Getenv("WHATSAPP_API_VERSION"), os.Getenv("WHATSAPP_PHONE_NUMBER_ID")
	if strings.ContainsAny(version+phone, "/?#\\") || phone == "" {
		database.Exec("UPDATE conversations SET status='failed' WHERE id=?", id)
		return
	}
	claim, err := database.Exec("UPDATE conversations SET status='sending' WHERE id=? AND status='queued'", id)
	if err != nil {
		return
	}
	claimed, err := claim.RowsAffected()
	if err != nil || claimed != 1 {
		return
	}
	req, err := http.NewRequestWithContext(ctx, "POST", "https://graph.facebook.com/"+version+"/"+phone+"/messages", bytes.NewReader(b))
	if err != nil {
		return
	}
	req.Header.Set("Authorization", "Bearer "+os.Getenv("WHATSAPP_ACCESS_TOKEN"))
	req.Header.Set("Content-Type", "application/json")
	res, err := providerHTTP.Do(req)
	if err != nil {
		database.Exec("UPDATE conversations SET status='uncertain' WHERE id=?", id)
		return
	}
	defer res.Body.Close()
	var result struct {
		Messages []struct{ ID string } `json:"messages"`
	}
	err = json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&result)
	if res.StatusCode >= 200 && res.StatusCode < 300 && err == nil && len(result.Messages) > 0 {
		database.Exec("UPDATE conversations SET provider_id=?,status='accepted' WHERE id=?", result.Messages[0].ID, id)
	} else {
		database.Exec("UPDATE conversations SET status='failed' WHERE id=?", id)
	}
}

func whatsAppTime(stamp string) string {
	seconds, err := strconv.ParseInt(stamp, 10, 64)
	if err != nil {
		return "invalid"
	}
	return time.Unix(seconds, 0).UTC().Format(time.RFC3339)
}

func subjectCaseReference(subject, reference string) bool {
	if reference == "" {
		return false
	}
	for _, word := range strings.FieldsFunc(subject, func(c rune) bool {
		return !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_')
	}) {
		if strings.EqualFold(word, reference) {
			return true
		}
	}
	return false
}
