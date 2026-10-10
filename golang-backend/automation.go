package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	webpush "github.com/SherClockHolmes/webpush-go"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

func notification(org, owner, kind, title, body, path, dedupe string) {
	if owner == "" {
		return
	}
	database.Exec("INSERT OR IGNORE INTO notifications VALUES(?,?,?,?,?,?,?,?,?,NULL)", newID("notice"), org, owner, kind, title, body, path, dedupe, utcNow())
}
func notificationWithDB(db DBRunner, org, owner, kind, title, body, path, dedupe string) {
	if owner == "" {
		return
	}
	db.Exec("INSERT OR IGNORE INTO notifications VALUES(?,?,?,?,?,?,?,?,?,NULL)", newID("notice"), org, owner, kind, title, body, path, dedupe, utcNow())
}
func generateReminders() {
	runner, r, state, close, err := workerRuntime(context.Background(), "")
	if err != nil {
		return
	}
	defer close()
	tx := runner.(*sql.Tx)
	original := state.cloneStore()
	s := state.cloneStore()
	changed := false
	type recurrence struct{ org, parent string }
	nextRecurrences := []recurrence{}
	zones := map[string]*time.Location{}
	orgs, err := tx.Query("SELECT id,timezone FROM organizations")
	if err == nil {
		for orgs.Next() {
			var id, zone string
			orgs.Scan(&id, &zone)
			location, e := time.LoadLocation(zone)
			if e != nil {
				location = time.UTC
			}
			zones[id] = location
		}
		orgs.Close()
	}
	orgDay := func(org string, offset int) string {
		zone := zones[org]
		if zone == nil {
			zone = time.UTC
		}
		return time.Now().In(zone).AddDate(0, 0, offset).Format("2006-01-02")
	}
	if err = remindPendingHandoffs(tx, s, orgDay); err != nil {
		return
	}
	today := time.Now().Format("2006-01-02")
	for _, t := range s.Tasks {
		if t.Status != "done" && t.ReminderDate != "" && t.ReminderDate <= orgDay(t.OrgID, 0) {
			notificationWithDB(tx, t.OrgID, t.OwnerID, "task", "Task reminder", t.Title, "/tasks", "task:"+t.ID+":"+t.ReminderDate)
		}
		if t.Status == "done" && t.Recurrence != "" && t.Recurrence != "none" {
			var count int
			tx.QueryRow("SELECT count(*) FROM workspace_entries WHERE category='recurrence' AND record_id=?", t.ID).Scan(&count)
			if count > 0 {
				continue
			}
			date, err := time.Parse("2006-01-02", t.DueDate)
			if err != nil {
				continue
			}
			switch t.Recurrence {
			case "daily":
				date = date.AddDate(0, 0, 1)
			case "weekly":
				date = date.AddDate(0, 0, 7)
			case "monthly":
				date = date.AddDate(0, 1, 0)
			}
			next := t
			next.ID = newID("t")
			next.Status = "todo"
			next.DueDate = date.Format("2006-01-02")
			next.ReminderDate = next.DueDate
			next.CreatedAt = today
			s.Tasks = append(s.Tasks, next)
			nextRecurrences = append(nextRecurrences, recurrence{t.OrgID, t.ID})
			changed = true
		}
	}
	for index, d := range s.Documents {
		if d.ExpiryDate != "" && d.ExpiryDate < orgDay(d.OrgID, 0) && d.Status != "Expired" {
			s.Documents[index].Status = "Expired"
			changed = true
		}
		if d.ExpiryDate != "" && d.ExpiryDate <= orgDay(d.OrgID, 30) {
			notificationWithDB(tx, d.OrgID, d.OwnerID, "document", "Document expiry", d.DocName+" expires "+d.ExpiryDate, "/cases/"+d.CaseID, "expiry:"+d.ID+":"+d.ExpiryDate)
		}
	}
	for _, c := range s.Cases {
		if c.NextDeadline != "" && c.NextDeadline <= orgDay(c.OrgID, 3) && c.CurrentStage != "Closed" && c.CurrentStage != "Approved" {
			notificationWithDB(tx, c.OrgID, c.OwnerID, "case", "Case deadline", c.CaseNumber+": "+c.NextDeadline, "/cases/"+c.ID, "deadline:"+c.ID+":"+c.NextDeadline)
		}
	}
	for _, inv := range s.Invoices {
		if inv.Dunning && inv.Balance > 0 && inv.DueDate != "" && inv.DueDate < orgDay(inv.OrgID, 0) {
			key := "dunning:" + inv.ID + ":" + orgDay(inv.OrgID, 0)
			var count int
			tx.QueryRow("SELECT count(*) FROM notifications WHERE user_id=? AND dedupe_key=?", inv.OwnerID, key).Scan(&count)
			if count > 0 {
				continue
			}
			notificationWithDB(tx, inv.OrgID, inv.OwnerID, "invoice", "Overdue invoice", inv.InvoiceNumber, "/invoices/"+inv.ID, key)
			for _, c := range s.Contacts {
				if c.ID == inv.ClientID && validEmail(c.Email) {
					tx.Exec("INSERT INTO mail_outbox(id,org_id,user_id,contact_id,invoice_id,kind,recipient,subject,body,status,created_at,next_attempt) VALUES(?,?,?,?,?,'reminder',?,?,?,'queued',?,?)", newID("mail"), inv.OrgID, inv.OwnerID, c.ID, inv.ID, c.Email, "Payment reminder: "+inv.InvoiceNumber, fmt.Sprintf("Your invoice balance is %s %.2f, due %s.", currencyOf(inv.Currency), inv.Balance, inv.DueDate), utcNow(), utcNow())
				}
			}
		}
	}

	if changed {
		state.restoreStore(s)
		state.applyComputedData()
		events := changesBetween(original, state.snapshot(), "System")
		if err = saveStoreDeltaTx(tx, original, state.snapshot(), events); err != nil {
			return
		}
		for _, rec := range nextRecurrences {
			if _, err = tx.Exec("INSERT INTO workspace_entries VALUES(?,?, 'recurrence','',?,'{}',?,?)", newID("entry"), rec.org, rec.parent, utcNow(), utcNow()); err != nil {
				return
			}
		}
		if err = state.queueChangeEvents(r, events); err != nil {
			return
		}
	}

	// Notification emails are opt-in per user; unique entries prevent repeated sends.
	rows, err := tx.Query("SELECT n.id,n.org_id,n.user_id,n.title,n.body FROM notifications n JOIN workspace_entries p ON p.user_id=n.user_id AND p.category='notification_preferences' AND json_extract(p.data,'$.email')=1 WHERE NOT EXISTS(SELECT 1 FROM workspace_entries e WHERE e.category='notification_email' AND e.record_id=n.id) LIMIT 20")
	if err != nil {
		return
	}
	type item struct{ id, org, user, title, body string }
	pending := []item{}
	for rows.Next() {
		var v item
		rows.Scan(&v.id, &v.org, &v.user, &v.title, &v.body)
		pending = append(pending, v)
	}
	rows.Close()
	for _, v := range pending {
		u, ok := state.findUserByID(v.user)
		if !ok {
			continue
		}
		tx.Exec("INSERT INTO mail_outbox(id,org_id,user_id,contact_id,invoice_id,kind,recipient,subject,body,status,created_at,next_attempt) VALUES(?,?,?,'','','notification',?,?,?,'queued',?,?)", newID("mail"), v.org, v.user, u.Email, v.title, v.body, utcNow(), utcNow())
		tx.Exec("INSERT INTO workspace_entries VALUES(?,?, 'notification_email',?,?, '{}',?,?)", newID("entry"), v.org, v.user, v.id, utcNow(), utcNow())
	}

	tx.Commit()
}
func publicURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil
}
func webhookClient() *http.Client {
	return &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return fmt.Errorf("redirects are disabled") }, Transport: &http.Transport{DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			if ip.IP.IsPrivate() || ip.IP.IsLoopback() || ip.IP.IsUnspecified() || ip.IP.IsLinkLocalUnicast() || ip.IP.IsLinkLocalMulticast() {
				return nil, fmt.Errorf("private network destinations are blocked")
			}
		}
		if len(ips) == 0 {
			return nil, fmt.Errorf("host unavailable")
		}
		return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
	}}}
}
func handleWebhookSettings(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	var req struct {
		URL     string
		Enabled bool
	}
	if !decodeRequest(w, r, &req) {
		return
	}
	if !publicURL(req.URL) {
		writeError(w, 400, "Use a public HTTPS webhook URL")
		return
	}
	secret := randomSecret()
	encrypted, _ := encryptSecret(secret)
	id, err := entry(r, "webhook", "", map[string]any{"url": req.URL, "enabled": req.Enabled, "secret": encrypted}, false)
	if err != nil {
		writeError(w, 500, "Could not save webhook")
		return
	}
	writeJSON(w, 201, map[string]string{"id": id, "secret": secret})
}
func deliverWebhooks() {
	database.Exec("UPDATE webhook_deliveries SET status='queued' WHERE status='sending' AND next_attempt<=?", utcNow())
	rows, err := database.Query("SELECT d.id,d.payload,d.attempts,w.data FROM webhook_deliveries d JOIN workspace_entries w ON w.id=d.webhook_id WHERE d.status='queued' AND d.next_attempt<=? LIMIT 10", utcNow())
	if err != nil {
		return
	}
	type item struct {
		id, payload, data string
		attempts          int
	}
	jobs := []item{}
	for rows.Next() {
		var v item
		rows.Scan(&v.id, &v.payload, &v.attempts, &v.data)
		jobs = append(jobs, v)
	}
	rows.Close()
	for _, v := range jobs {
		if !claimDelivery("webhook_deliveries", v.id) {
			continue
		}
		var config struct {
			URL, Secret string
			Enabled     bool
		}
		json.Unmarshal([]byte(v.data), &config)
		status := "failed"
		problem := "Webhook disabled"
		if config.Enabled && publicURL(config.URL) {
			secret, err := decryptSecret(config.Secret)
			if err == nil {
				stamp := fmt.Sprint(time.Now().Unix())
				mac := hmac.New(sha256.New, []byte(secret))
				mac.Write([]byte(stamp + "." + v.payload))
				req, _ := http.NewRequest("POST", config.URL, strings.NewReader(v.payload))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-ECC-Timestamp", stamp)
				req.Header.Set("X-ECC-Signature", hex.EncodeToString(mac.Sum(nil)))
				req.Header.Set("X-ECC-Delivery", v.id)
				response, e := webhookClient().Do(req)
				if e == nil {
					io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
					response.Body.Close()
					if response.StatusCode >= 200 && response.StatusCode < 300 {
						status = "delivered"
						problem = ""
					} else {
						problem = "Endpoint returned " + response.Status
					}
				} else {
					problem = "Endpoint unavailable"
				}
				if status != "delivered" && v.attempts < 4 {
					status = "queued"
				}
			}
		}
		database.Exec("UPDATE webhook_deliveries SET status=?,attempts=attempts+1,last_error=?,next_attempt=? WHERE id=?", status, problem, time.Now().Add(time.Duration((v.attempts+1)*(v.attempts+1))*time.Minute).UTC().Format(time.RFC3339), v.id)
	}
}
func allowedPush(raw string) bool {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" {
		return false
	}
	h := u.Hostname()
	return h == "fcm.googleapis.com" || h == "updates.push.services.mozilla.com" || strings.HasSuffix(h, ".push.apple.com") || h == "web.push.apple.com"
}
func handlePushSubscription(w http.ResponseWriter, r *http.Request) {
	var sub webpush.Subscription
	if !decodeRequest(w, r, &sub) {
		return
	}
	if !allowedPush(sub.Endpoint) || sub.Keys.Auth == "" || sub.Keys.P256dh == "" {
		writeError(w, 400, "Unsupported push subscription")
		return
	}
	id, err := entry(r, "push_subscription", "", sub, true)
	if err != nil {
		writeError(w, 500, "Could not save subscription")
		return
	}
	writeJSON(w, 201, map[string]string{"id": id})
}
func deliverPushNotifications() {
	pub, private := os.Getenv("VAPID_PUBLIC_KEY"), os.Getenv("VAPID_PRIVATE_KEY")
	if pub == "" || private == "" {
		return
	}
	rows, err := database.Query("SELECT n.id,n.title,n.body,n.path,s.id,s.data,s.org_id,s.user_id FROM notifications n JOIN workspace_entries s ON s.user_id=n.user_id AND s.category='push_subscription' WHERE NOT EXISTS(SELECT 1 FROM workspace_entries e WHERE e.category='push_delivered' AND e.record_id=n.id||':'||s.id) LIMIT 10")
	if err != nil {
		return
	}
	type job struct{ id, title, body, path, subID, data, org, user string }
	jobs := []job{}
	for rows.Next() {
		var j job
		rows.Scan(&j.id, &j.title, &j.body, &j.path, &j.subID, &j.data, &j.org, &j.user)
		jobs = append(jobs, j)
	}
	rows.Close()
	for _, j := range jobs {
		var sub webpush.Subscription
		if json.Unmarshal([]byte(j.data), &sub) != nil || !allowedPush(sub.Endpoint) {
			continue
		}
		claimID := "push_claim_" + hashSecret(j.org+":"+j.id+":"+j.subID)
		result, claimErr := database.Exec("INSERT OR IGNORE INTO workspace_entries VALUES(?,?, 'push_claim',?,?, '{}',?,?)", claimID, j.org, j.user, j.id+":"+j.subID, utcNow(), utcNow())
		if claimErr != nil {
			continue
		}
		claimed, _ := result.RowsAffected()
		if claimed != 1 {
			continue
		}
		res, err := webpush.SendNotification([]byte(mustJSON(map[string]string{"title": j.title, "body": j.body, "path": j.path})), &sub, &webpush.Options{Subscriber: os.Getenv("VAPID_SUBJECT"), VAPIDPublicKey: pub, VAPIDPrivateKey: private, TTL: 3600, HTTPClient: webhookClient()})
		if err != nil {
			database.Exec("UPDATE workspace_entries SET data=? WHERE id=?", mustJSON(map[string]string{"status": "delivery_unknown"}), claimID)
			continue
		}
		res.Body.Close()
		if res.StatusCode == 404 || res.StatusCode == 410 {
			database.Exec("DELETE FROM workspace_entries WHERE id=?", j.subID)
		}
		if res.StatusCode < 300 {
			database.Exec("INSERT INTO workspace_entries VALUES(?,?, 'push_delivered',?,?, '{}',?,?)", newID("entry"), j.org, j.user, j.id+":"+j.subID, utcNow(), utcNow())
		}
	}
}
func handleNotificationPreferences(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email bool `json:"email"`
	}
	if !decodeRequest(w, r, &req) {
		return
	}
	storeDB(r).Exec("DELETE FROM workspace_entries WHERE user_id=? AND category='notification_preferences'", currentUser(r).ID)
	_, err := entry(r, "notification_preferences", "", req, true)
	if err != nil {
		writeError(w, 500, "Could not save preferences")
		return
	}
	writeJSON(w, 200, statusResponse{"ok"})
}

var _ = bytes.NewBuffer

func handleManageWebhook(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	var req struct{ Enabled bool }
	if !decodeRequest(w, r, &req) {
		return
	}
	result, err := storeDB(r).Exec("UPDATE workspace_entries SET data=json_set(data,'$.enabled',json(?)),updated_at=? WHERE id=? AND org_id=? AND category='webhook'", fmt.Sprint(req.Enabled), utcNow(), r.PathValue("webhook"), currentUser(r).OrgID)
	if err != nil {
		writeError(w, 500, "Could not update webhook")
		return
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		writeError(w, 404, "Webhook not found")
		return
	}
	writeJSON(w, 200, statusResponse{"ok"})
}
func handleRetryWebhook(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	result, err := storeDB(r).Exec("UPDATE webhook_deliveries SET status='queued',attempts=0,next_attempt=? WHERE id=? AND org_id=? AND status='failed'", utcNow(), r.PathValue("delivery"), currentUser(r).OrgID)
	if err != nil {
		writeError(w, 500, "Could not retry webhook")
		return
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		writeError(w, 404, "Failed delivery not found")
		return
	}
	writeJSON(w, 200, statusResponse{"ok"})
}
