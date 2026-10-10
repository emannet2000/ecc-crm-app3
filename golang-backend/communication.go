package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

func integrationStatus() map[string]any {
	return map[string]any{"smtp": os.Getenv("SMTP_HOST") != "", "payments": os.Getenv("STRIPE_SECRET_KEY") != "", "sso": os.Getenv("OIDC_ISSUER_URL") != "", "saml": os.Getenv("SAML_METADATA_URL") != "", "push": os.Getenv("VAPID_PUBLIC_KEY") != "", "emailMode": "queued"}
}
func queueMail(r *http.Request, user User, kind, recipient, subject, body, contactID, invoiceID string) error {
	if !validEmail(recipient) || strings.ContainsAny(subject, "\r\n") || len(subject) > 200 || len(body) > 100000 {
		return fmt.Errorf("invalid email")
	}
	_, err := storeDB(r).Exec(`INSERT INTO mail_outbox(id,org_id,user_id,contact_id,invoice_id,kind,recipient,subject,body,status,created_at) VALUES(?,?,?,?,?,?,?,?,?,'queued',?)`, newID("mail"), user.OrgID, user.ID, contactID, invoiceID, kind, recipient, subject, body, utcNow())
	return err
}
func (state *runtimeStore) handleComposeMail(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	var req struct{ Recipient, Subject, Body, ContactID string }
	if !decodeRequest(w, r, &req) {
		return
	}
	if !validEmail(req.Recipient) || strings.TrimSpace(req.Subject) == "" || strings.TrimSpace(req.Body) == "" {
		writeError(w, 400, "Enter a recipient, subject, and message")
		return
	}
	var contact Contact
	if req.ContactID != "" {
		found := false
		state.mu.RLock()
		for _, candidate := range state.contacts {
			if candidate.ID == req.ContactID {
				contact = candidate
				found = true
				break
			}
		}
		state.mu.RUnlock()
		if !found {
			writeError(w, 404, "Contact not found")
			return
		}
		if !canWrite(contact.RecordScope, user) {
			writeError(w, 403, "You cannot send mail for this contact")
			return
		}
	} else if !isManager(user) {
		writeError(w, 403, "Choose a contact you own")
		return
	}
	if err := queueMail(r, user, "client", req.Recipient, req.Subject, req.Body, req.ContactID, ""); err != nil {
		writeError(w, 400, "Could not queue email")
		return
	}
	if req.ContactID != "" {
		state.mu.Lock()
		state.activities = append(state.activities, Activity{RecordScope: newRecordScope(r), ID: newID("a"), ContactID: req.ContactID, Kind: "email", Title: req.Subject, Body: "Email queued to " + req.Recipient + "\n\n" + req.Body, OccurredAt: utcNow(), CreatedBy: user.Name, CreatedAt: utcNow()})
		for i := range state.contacts {
			if state.contacts[i].ID == req.ContactID {
				state.contacts[i].LastContact = organizationToday(r)
			}
		}
		state.mu.Unlock()
	}
	writeJSON(w, 202, map[string]string{"message": "Email queued. Delivery status is shown in the outbox."})
}

func handleComposeMail(w http.ResponseWriter, r *http.Request) {
	state, release := runtimeForCall(r)
	defer release()
	state.handleComposeMail(w, r)
}
func handleRetryMail(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	result, err := storeDB(r).Exec(`UPDATE mail_outbox SET status='queued',attempts=0,last_error='',next_attempt='' WHERE id=? AND org_id=? AND (user_id=? OR ?='admin') AND status IN ('failed','waiting_configuration')`, r.PathValue("message"), user.OrgID, user.ID, user.Role)
	if err != nil {
		writeError(w, 500, "Could not retry email")
		return
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		writeError(w, 404, "Retryable email not found")
		return
	}
	writeJSON(w, 200, statusResponse{"ok"})
}
func handleReadNotification(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("notification")
	query := "UPDATE notifications SET read_at=? WHERE user_id=?"
	args := []any{utcNow(), currentUser(r).ID}
	if id != "all" {
		query += " AND id=?"
		args = append(args, id)
	}
	if _, err := storeDB(r).Exec(query, args...); err != nil {
		writeError(w, 500, "Could not update notifications")
		return
	}
	writeJSON(w, 200, statusResponse{"ok"})
}
func (state *runtimeStore) queueChangeEvents(r *http.Request, events []AuditEvent) error {
	for _, event := range events {
		payload, _ := json.Marshal(event)
		owner, _ := state.findUserByID(event.Scope.OwnerID)
		if event.Scope.OwnerID != "" && owner.Email != event.Actor {
			if _, err := storeDB(r).Exec("INSERT OR IGNORE INTO notifications VALUES(?,?,?,?,?,?,?,?,?,NULL)", newID("notice"), event.OrgID, event.Scope.OwnerID, "record", event.Label+" "+event.Action, "Changed by "+event.Actor, "/"+event.Entity+"/"+event.RecordID, newID("event"), utcNow()); err != nil {
				return err
			}
		}
		webhooks, err := queryObjects(r, "SELECT id FROM workspace_entries WHERE org_id=? AND category='webhook' AND json_extract(data,'$.enabled')=1", event.OrgID)
		if err != nil {
			return err
		}
		for _, webhook := range webhooks {
			if _, err = storeDB(r).Exec(`INSERT INTO webhook_deliveries(id,org_id,webhook_id,payload,status,created_at,next_attempt) VALUES(?,?,?,?,'queued',?,?)`, newID("delivery"), event.OrgID, webhook["id"], string(payload), utcNow(), utcNow()); err != nil {
				return err
			}
		}
	}
	return nil
}

func queueChangeEvents(r *http.Request, events []AuditEvent) error {
	state, release := runtimeForReadCall(r)
	defer release()
	return state.queueChangeEvents(r, events)
}
func organizationToday(r *http.Request) string {
	var zone string
	storeDB(r).QueryRow("SELECT timezone FROM organizations WHERE id=?", currentUser(r).OrgID).Scan(&zone)
	location, err := time.LoadLocation(zone)
	if err != nil {
		location = time.UTC
	}
	return time.Now().In(location).Format("2006-01-02")
}

type smtpSettings struct{ Host, Port, Username, Password, From, TLSMode string }

func smtpConfiguration(org string) smtpSettings {
	config := smtpSettings{os.Getenv("SMTP_HOST"), os.Getenv("SMTP_PORT"), os.Getenv("SMTP_USER"), os.Getenv("SMTP_PASSWORD"), os.Getenv("SMTP_FROM"), os.Getenv("SMTP_TLS_MODE")}
	var payload string
	if database.QueryRow("SELECT data FROM workspace_entries WHERE org_id=? AND category='integration_secret' AND record_id='smtp'", org).Scan(&payload) == nil {
		var saved smtpSettings
		if json.Unmarshal([]byte(payload), &saved) == nil {
			if password, err := decryptSecret(saved.Password); err == nil {
				saved.Password = password
			}
			config = saved
		}
	}
	if config.Port == "" {
		config.Port = "587"
	}
	if config.TLSMode == "" {
		config.TLSMode = "starttls"
	}
	return config
}
func deliverSMTP(config smtpSettings, recipient, subject, body string, attachment []byte) error {
	if config.Host == "" {
		return fmt.Errorf("SMTP is not configured")
	}
	from, err := mail.ParseAddress(config.From)
	if err != nil {
		return fmt.Errorf("SMTP sender is invalid")
	}
	address := net.JoinHostPort(config.Host, config.Port)
	tlsConfig := &tls.Config{ServerName: config.Host, MinVersion: tls.VersionTLS12}
	var connection net.Conn
	if config.TLSMode == "implicit" {
		connection, err = tls.DialWithDialer(&net.Dialer{Timeout: 15 * time.Second}, "tcp", address, tlsConfig)
	} else {
		connection, err = net.DialTimeout("tcp", address, 15*time.Second)
	}
	if err != nil {
		return err
	}
	defer connection.Close()
	connection.SetDeadline(time.Now().Add(30 * time.Second))
	client, err := smtp.NewClient(connection, config.Host)
	if err != nil {
		return err
	}
	defer client.Close()
	if config.TLSMode != "implicit" {
		if config.TLSMode == "local" {
			host, _, _ := net.SplitHostPort(connection.RemoteAddr().String())
			ip := net.ParseIP(host)
			if ip == nil || !ip.IsLoopback() {
				return fmt.Errorf("Unencrypted SMTP is allowed only on loopback")
			}
		} else {
			if err = client.StartTLS(tlsConfig); err != nil {
				return err
			}
		}
	}
	if config.Username != "" {
		if err = client.Auth(smtp.PlainAuth("", config.Username, config.Password, config.Host)); err != nil {
			return err
		}
	}
	if err = client.Mail(from.Address); err != nil {
		return err
	}
	if err = client.Rcpt(recipient); err != nil {
		return err
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	var message bytes.Buffer
	fmt.Fprintf(&message, "From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\n", config.From, recipient, mime.QEncoding.Encode("utf-8", subject))
	if len(attachment) == 0 {
		message.WriteString("Content-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: base64\r\n\r\n")
		message.WriteString(wrapBase64([]byte(body)))
	} else {
		boundary := "ecc_" + randomSecret()
		fmt.Fprintf(&message, "Content-Type: multipart/mixed; boundary=%q\r\n\r\n--%s\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: base64\r\n\r\n%s\r\n--%s\r\nContent-Type: application/pdf\r\nContent-Disposition: attachment; filename=\"invoice.pdf\"\r\nContent-Transfer-Encoding: base64\r\n\r\n%s\r\n--%s--\r\n", boundary, boundary, wrapBase64([]byte(body)), boundary, wrapBase64(attachment), boundary)
	}
	if _, err = writer.Write(message.Bytes()); err != nil {
		return err
	}
	if err = writer.Close(); err != nil {
		return err
	}
	return client.Quit()
}
func wrapBase64(data []byte) string {
	encoded := base64.StdEncoding.EncodeToString(data)
	var out strings.Builder
	for len(encoded) > 76 {
		out.WriteString(encoded[:76] + "\r\n")
		encoded = encoded[76:]
	}
	out.WriteString(encoded + "\r\n")
	return out.String()
}

var workerHeartbeatMu sync.RWMutex
var workerHeartbeats = map[string]time.Time{}

func markWorkerRun(names ...string) {
	workerHeartbeatMu.Lock()
	defer workerHeartbeatMu.Unlock()
	for _, name := range names {
		workerHeartbeats[name] = time.Now().UTC()
	}
}

func startWorkers(ctx context.Context) func() {
	var workers sync.WaitGroup
	workers.Add(1)
	go func() {
		defer workers.Done()
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				markWorkerRun("mail-delivery", "reminders", "follow-up-automation", "webhooks", "push-notifications")
				deliverMailOutbox()
				generateReminders()
				runFollowupRules()
				deliverWebhooks()
				deliverPushNotifications()
			}
		}
	}()
	return workers.Wait
}
func deliverMailOutbox() {
	database.Exec("UPDATE mail_outbox SET status='delivery_unknown',last_error='Delivery was interrupted; verify with the provider before retrying' WHERE status='sending' AND next_attempt<=?", utcNow())
	rows, err := database.Query(`SELECT id,org_id,recipient,subject,body,invoice_id,attempts FROM mail_outbox WHERE status IN ('queued','waiting_configuration') AND (next_attempt='' OR next_attempt<=?) ORDER BY created_at LIMIT 10`, utcNow())
	if err != nil {
		return
	}
	type delivery struct {
		id, org, recipient, subject, body, invoice string
		attempts                                   int
	}
	jobs := []delivery{}
	for rows.Next() {
		var job delivery
		if rows.Scan(&job.id, &job.org, &job.recipient, &job.subject, &job.body, &job.invoice, &job.attempts) == nil {
			jobs = append(jobs, job)
		}
	}
	rows.Close()
	for _, job := range jobs {
		if !claimDelivery("mail_outbox", job.id) {
			continue
		}
		config := smtpConfiguration(job.org)
		if config.Host == "" {
			database.Exec("UPDATE mail_outbox SET status='waiting_configuration',last_error='Configure SMTP delivery' WHERE id=?", job.id)
			continue
		}
		var attachment []byte
		if job.invoice != "" {
			var payload []byte
			if database.QueryRow("SELECT data FROM invoices WHERE id=? AND json_extract(data,'$.orgId')=?", job.invoice, job.org).Scan(&payload) == nil {
				var invoice Invoice
				json.Unmarshal(payload, &invoice)
				attachment, _ = invoicePDF(invoice)
			}
		}
		err := deliverSMTP(config, job.recipient, job.subject, job.body, attachment)
		if err == nil {
			database.Exec("UPDATE mail_outbox SET status='sent',sent_at=?,attempts=attempts+1,last_error='' WHERE id=?", utcNow(), job.id)
		} else {
			attempts := job.attempts + 1
			status := "queued"
			if attempts >= 5 {
				status = "failed"
			}
			database.Exec("UPDATE mail_outbox SET status=?,attempts=?,last_error=?,next_attempt=? WHERE id=?", status, attempts, "Delivery failed; verify SMTP settings", time.Now().Add(time.Duration(attempts*attempts)*time.Minute).UTC().Format(time.RFC3339), job.id)
		}
	}
}
func handleSMTPSettings(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	var req smtpSettings
	if !decodeRequest(w, r, &req) {
		return
	}
	if req.Host == "" || !validEmail(req.From) {
		writeError(w, 400, "Enter an SMTP host and valid sender email")
		return
	}
	port, err := strconv.Atoi(req.Port)
	if err != nil || port < 1 || port > 65535 {
		writeError(w, 400, "Enter a valid SMTP port")
		return
	}
	if req.TLSMode != "starttls" && req.TLSMode != "implicit" && req.TLSMode != "local" {
		writeError(w, 400, "Choose STARTTLS or implicit TLS")
		return
	}
	if req.Password == "" {
		var old string
		if storeDB(r).QueryRow("SELECT data FROM workspace_entries WHERE id=?", "smtp_"+currentUser(r).OrgID).Scan(&old) == nil {
			var config smtpSettings
			if json.Unmarshal([]byte(old), &config) == nil {
				req.Password, _ = decryptSecret(config.Password)
			}
		}
	}
	req.Password, err = encryptSecret(req.Password)
	if err != nil {
		writeError(w, 500, "Could not protect credentials")
		return
	}
	payload, _ := json.Marshal(req)
	_, err = storeDB(r).Exec(`INSERT INTO workspace_entries(id,org_id,category,user_id,record_id,data,created_at,updated_at) VALUES(?,?,'integration_secret','','smtp',?,?,?) ON CONFLICT(id) DO UPDATE SET data=excluded.data,updated_at=excluded.updated_at`, "smtp_"+currentUser(r).OrgID, currentUser(r).OrgID, string(payload), utcNow(), utcNow())
	if err != nil {
		writeError(w, 500, "Could not save SMTP settings")
		return
	}
	writeJSON(w, 200, statusResponse{"ok"})
}

func integrationStatusFor(r *http.Request) map[string]any {
	features := integrationStatus()
	var data string
	if storeDB(r).QueryRow("SELECT data FROM workspace_entries WHERE id=?", "smtp_"+currentUser(r).OrgID).Scan(&data) == nil {
		var conf smtpSettings
		json.Unmarshal([]byte(data), &conf)
		features["smtp"] = conf.Host != ""
	}
	features["ai"] = assistantEnabled(r)
	features["whatsapp"] = currentUser(r).OrgID == os.Getenv("WHATSAPP_ORG_ID") && os.Getenv("WHATSAPP_ACCESS_TOKEN") != ""
	features["inboundEmail"] = os.Getenv("INBOUND_EMAIL_SECRET") != ""
	features["mailboxSync"] = currentUser(r).OrgID == os.Getenv("GRAPH_ORG_ID")
	features["vapidPublicKey"] = os.Getenv("VAPID_PUBLIC_KEY")
	return features
}
