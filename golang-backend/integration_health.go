package main

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

type integrationHealth struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	State       string `json:"state"`
	Message     string `json:"message"`
	CheckedAt   string `json:"checkedAt,omitempty"`
	Fingerprint string `json:"-"`
	TestMode    *bool  `json:"testMode,omitempty"`
}
type savedIntegrationTest struct {
	State       string `json:"state"`
	Message     string `json:"message"`
	Fingerprint string `json:"fingerprint"`
}

var integrationProbeHTTP = &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

func integrationConfiguration(r *http.Request, id string) integrationHealth {
	h := integrationHealth{ID: id, State: "needs_configuration"}
	values := []string{}
	switch id {
	case "email":
		h.Name = "Email"
		c := smtpConfiguration(currentUser(r).OrgID)
		values = []string{c.Host, c.Port, c.Username, c.Password, c.From, c.TLSMode}
		h.Message = "Save an SMTP host, sender and TLS settings using Connect SMTP below."
		if c.Host != "" && validEmail(c.From) {
			h.State = "configured"
			h.Message = "SMTP is configured. Test connection to verify TLS and authentication."
		}
	case "whatsapp":
		h.Name = "WhatsApp"
		values = []string{os.Getenv("WHATSAPP_ORG_ID"), os.Getenv("WHATSAPP_ACCESS_TOKEN"), os.Getenv("WHATSAPP_PHONE_NUMBER_ID"), os.Getenv("WHATSAPP_API_VERSION"), os.Getenv("WHATSAPP_APP_SECRET"), os.Getenv("WHATSAPP_VERIFY_TOKEN")}
		h.Message = "Configure workspace, token, phone number, API version and signed webhooks in deployment settings."
		if values[0] == currentUser(r).OrgID && values[1] != "" && values[2] != "" && values[3] != "" {
			h.State = "configured"
			h.Message = "Credentials configured. Test verifies phone-number access; delivery needs signed webhook callbacks."
		}
	case "payments":
		h.Name = "Payments"
		values = []string{os.Getenv("STRIPE_SECRET_KEY"), os.Getenv("STRIPE_WEBHOOK_SECRET")}
		testMode := strings.HasPrefix(values[0], "sk_test_") || strings.HasPrefix(values[0], "rk_test_")
		h.TestMode = &testMode
		h.Message = "Configure STRIPE_SECRET_KEY and STRIPE_WEBHOOK_SECRET in deployment settings."
		if values[0] != "" {
			h.State = "configured"
			h.Message = "Stripe key configured. Test verifies account access; payment completion needs the webhook secret."
		}
	case "ai":
		h.Name = "AI"
		settings := loadAISettings(r)
		values = []string{os.Getenv("OPENAI_API_KEY"), settings.Model, fmt.Sprint(settings.Enabled)}
		h.Message = "Configure an AI provider key and model, then enable AI for this workspace."
		if values[0] != "" && values[1] != "" {
			h.State = "disabled"
			h.Message = "Provider configured; enable AI for this workspace below."
			if settings.Enabled {
				h.State = "configured"
				h.Message = "AI enabled. Test verifies access to the selected model without sending CRM records."
			}
		}
	}
	sum := sha256.Sum256([]byte(mustJSON(values)))
	h.Fingerprint = hex.EncodeToString(sum[:])
	return h
}
func handleIntegrationHealth(w http.ResponseWriter, r *http.Request) {
	items := []integrationHealth{}
	for _, id := range []string{"email", "whatsapp", "payments", "ai"} {
		h := integrationConfiguration(r, id)
		var payload, at string
		if storeDB(r).QueryRow("SELECT data,updated_at FROM workspace_entries WHERE org_id=? AND category='integration_test' AND record_id=?", currentUser(r).OrgID, id).Scan(&payload, &at) == nil {
			var saved savedIntegrationTest
			if json.Unmarshal([]byte(payload), &saved) == nil && saved.Fingerprint == h.Fingerprint && h.State == "configured" {
				h.State = saved.State
				h.Message = saved.Message
				h.CheckedAt = at
				if tested, err := time.Parse(time.RFC3339, at); err == nil && time.Since(tested) > 24*time.Hour {
					h.State = "configured"
					h.Message = "Last connection check is older than 24 hours. Test again to verify current access."
				}
			}
		}
		items = append(items, h)
	}
	writeJSON(w, 200, map[string]any{"integrations": items, "canTest": isAdmin(currentUser(r))})
}
func testSMTPConnection(ctx context.Context, c smtpSettings) error {
	if !validEmail(c.From) {
		return fmt.Errorf("invalid sender")
	}
	if c.TLSMode != "implicit" && c.TLSMode != "starttls" && c.TLSMode != "local" {
		return fmt.Errorf("invalid TLS mode")
	}
	tlsConfig := &tls.Config{ServerName: c.Host, MinVersion: tls.VersionTLS12}
	dialer := &net.Dialer{Timeout: 8 * time.Second}
	var conn net.Conn
	var err error
	if c.TLSMode == "implicit" {
		conn, err = (&tls.Dialer{NetDialer: dialer, Config: tlsConfig}).DialContext(ctx, "tcp", net.JoinHostPort(c.Host, c.Port))
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", net.JoinHostPort(c.Host, c.Port))
	}
	if err != nil {
		return err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	client, err := smtp.NewClient(conn, c.Host)
	if err != nil {
		return err
	}
	defer client.Close()
	if c.TLSMode == "local" {
		host, _, _ := net.SplitHostPort(conn.RemoteAddr().String())
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return fmt.Errorf("local SMTP must use loopback")
		}
	} else if c.TLSMode == "starttls" {
		if err = client.StartTLS(tlsConfig); err != nil {
			return err
		}
	}
	if c.Username != "" {
		if err = client.Auth(smtp.PlainAuth("", c.Username, c.Password, c.Host)); err != nil {
			return err
		}
	}
	return client.Noop()
}
func probeIntegration(r *http.Request, id string) (string, error) {
	if id == "email" {
		return "SMTP connection, TLS and authentication succeeded. No email was sent.", testSMTPConnection(r.Context(), smtpConfiguration(currentUser(r).OrgID))
	}
	endpoint, key := "", ""
	switch id {
	case "ai":
		endpoint = "https://api.openai.com/v1/models/" + url.PathEscape(loadAISettings(r).Model)
		key = os.Getenv("OPENAI_API_KEY")
	case "payments":
		endpoint = "https://api.stripe.com/v1/account"
		key = os.Getenv("STRIPE_SECRET_KEY")
	case "whatsapp":
		version, phone := os.Getenv("WHATSAPP_API_VERSION"), os.Getenv("WHATSAPP_PHONE_NUMBER_ID")
		if !regexp.MustCompile(`^v[0-9]+\.[0-9]+$`).MatchString(version) || !regexp.MustCompile(`^[0-9]+$`).MatchString(phone) {
			return "", fmt.Errorf("invalid WhatsApp configuration")
		}
		endpoint = "https://graph.facebook.com/" + version + "/" + phone + "?fields=id,display_phone_number"
		key = os.Getenv("WHATSAPP_ACCESS_TOKEN")
	default:
		return "", fmt.Errorf("unknown integration")
	}
	req, err := http.NewRequestWithContext(r.Context(), "GET", endpoint, nil)
	if err != nil {
		return "", err
	}
	if id == "payments" {
		req.SetBasicAuth(key, "")
	} else {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	res, err := integrationProbeHTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	io.Copy(io.Discard, io.LimitReader(res.Body, 1<<20))
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", fmt.Errorf("provider returned HTTP %d", res.StatusCode)
	}
	switch id {
	case "ai":
		return "Connected: credentials can access the selected model. Generation and quota still need an AI request.", nil
	case "payments":
		return "Connected: Stripe account access verified. Test checkout and signed callbacks before accepting payments.", nil
	default:
		return "Connected: WhatsApp phone-number access verified. Message delivery still requires signed callbacks.", nil
	}
}
func handleIntegrationTest(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id := r.PathValue("integration")
	if !strings.Contains("|email|whatsapp|payments|ai|", "|"+id+"|") || id == "" {
		writeError(w, 404, "Integration not found")
		return
	}
	h := integrationConfiguration(r, id)
	if h.State != "configured" {
		writeJSON(w, 200, h)
		return
	}
	message, err := probeIntegration(r, id)
	h.CheckedAt = utcNow()
	if err != nil {
		h.State = "failing"
		h.Message = "Connection check failed. Verify credentials, provider access and network connectivity, then retry."
		if id == "email" {
			h.Message = "SMTP check failed. Verify host, port, TLS mode, sender and login credentials, then retry."
		}
	} else {
		h.State = "connected"
		h.Message = message
	}
	saved := savedIntegrationTest{State: h.State, Message: h.Message, Fingerprint: h.Fingerprint}
	_, err = database.Exec(`INSERT INTO workspace_entries(id,org_id,category,user_id,record_id,data,created_at,updated_at) VALUES(?,?,'integration_test',?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET data=excluded.data,updated_at=excluded.updated_at`, "integration_test_"+currentUser(r).OrgID+"_"+id, currentUser(r).OrgID, currentUser(r).ID, id, mustJSON(saved), h.CheckedAt, h.CheckedAt)
	if err != nil {
		writeError(w, 500, "Check completed but could not save its status; retry")
		return
	}
	writeJSON(w, 200, h)
}
