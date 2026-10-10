package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

var graphAccessToken string
var graphTokenExpiry time.Time

func graphToken(ctx context.Context) (string, error) {
	if os.Getenv("GRAPH_REFRESH_TOKEN") == "" {
		token := os.Getenv("GRAPH_ACCESS_TOKEN")
		if token == "" {
			return "", fmt.Errorf("Microsoft Graph credentials are missing")
		}
		return token, nil
	}
	if graphAccessToken != "" && time.Now().Before(graphTokenExpiry) {
		return graphAccessToken, nil
	}
	tenant := os.Getenv("GRAPH_TENANT_ID")
	if tenant == "" || strings.ContainsAny(tenant, "/?#\\") {
		return "", fmt.Errorf("Configure GRAPH_TENANT_ID")
	}
	form := url.Values{"client_id": {os.Getenv("GRAPH_CLIENT_ID")}, "client_secret": {os.Getenv("GRAPH_CLIENT_SECRET")}, "refresh_token": {os.Getenv("GRAPH_REFRESH_TOKEN")}, "grant_type": {"refresh_token"}, "scope": {"https://graph.microsoft.com/Mail.Read offline_access"}}
	req, err := http.NewRequestWithContext(ctx, "POST", "https://login.microsoftonline.com/"+tenant+"/oauth2/v2.0/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := providerHTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("Mailbox authorization unavailable")
	}
	defer res.Body.Close()
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	err = json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&out)
	if err != nil || res.StatusCode != 200 || out.AccessToken == "" {
		return "", fmt.Errorf("Mailbox authorization failed; reconnect the Microsoft account")
	}
	graphAccessToken = out.AccessToken
	graphTokenExpiry = time.Now().Add(time.Duration(out.ExpiresIn)*time.Second - time.Minute)
	return graphAccessToken, nil
}
func validGraphLink(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host == "graph.microsoft.com" && u.User == nil && strings.HasPrefix(u.Path, "/v1.0/")
}
func setSyncStatus(status, problem string) {
	database.Exec("INSERT INTO app_meta(key,value) VALUES('graph_sync_status',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", mustJSON(map[string]string{"status": status, "detail": problem, "at": utcNow()}))
}
func handleMailboxStatus(w http.ResponseWriter, r *http.Request) {
	if !isManager(currentUser(r)) {
		writeError(w, 403, "Manager access is required")
		return
	}
	if currentUser(r).OrgID != os.Getenv("GRAPH_ORG_ID") {
		writeJSON(w, 200, map[string]string{"status": "not_configured"})
		return
	}
	var data string
	if storeDB(r).QueryRow("SELECT value FROM app_meta WHERE key='graph_sync_status'").Scan(&data) != nil {
		writeJSON(w, 200, map[string]string{"status": "waiting"})
		return
	}
	var out any
	json.Unmarshal([]byte(data), &out)
	writeJSON(w, 200, out)
}
func syncMailbox(ctx context.Context) {
	org := os.Getenv("GRAPH_ORG_ID")
	if org == "" {
		return
	}
	token, err := graphToken(ctx)
	if err != nil {
		setSyncStatus("failed", err.Error())
		return
	}
	var endpoint string
	database.QueryRow("SELECT value FROM app_meta WHERE key=?", "graph_cursor_"+org).Scan(&endpoint)
	if endpoint == "" {
		endpoint = "https://graph.microsoft.com/v1.0/me/mailFolders/inbox/messages/delta?$select=id,from,subject,body,receivedDateTime&$top=50"
	}
	if !validGraphLink(endpoint) {
		setSyncStatus("failed", "Invalid mailbox cursor")
		return
	}
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Prefer", `outlook.body-content-type="text"`)
	res, err := providerHTTP.Do(req)
	if err != nil {
		setSyncStatus("failed", "Mailbox service unavailable")
		return
	}
	defer res.Body.Close()
	if res.StatusCode == 410 {
		database.Exec("DELETE FROM app_meta WHERE key=?", "graph_cursor_"+org)
		setSyncStatus("resyncing", "Mailbox cursor expired; deduplicated resync starts on the next cycle")
		return
	}
	if res.StatusCode != 200 {
		if res.StatusCode == 401 {
			graphTokenExpiry = time.Time{}
		}
		setSyncStatus("failed", fmt.Sprintf("Microsoft Graph returned HTTP %d", res.StatusCode))
		return
	}
	var page struct {
		Next  string `json:"@odata.nextLink"`
		Delta string `json:"@odata.deltaLink"`
		Value []struct {
			ID, Subject, ReceivedDateTime string
			From                          struct {
				EmailAddress struct{ Address string } `json:"emailAddress"`
			}
			Body    struct{ ContentType, Content string }
			Removed json.RawMessage `json:"@removed"`
		} `json:"value"`
	}
	if json.NewDecoder(io.LimitReader(res.Body, 8<<20)).Decode(&page) != nil {
		setSyncStatus("failed", "Unreadable mailbox response")
		return
	}
	next := page.Next
	if next == "" {
		next = page.Delta
	}
	if !validGraphLink(next) {
		setSyncStatus("failed", "Mailbox response had no valid next cursor")
		return
	}
	runner, r, state, close, err := workerRuntime(ctx, org)
	if err != nil {
		setSyncStatus("failed", "Database unavailable")
		return
	}
	defer close()
	tx := runner.(*sql.Tx)
	before := state.cloneStore()

	for _, message := range page.Value {
		if len(message.Removed) > 0 {
			continue
		}
		body := message.Body.Content
		if len(body) > 100000 {
			body = body[:100000]
		}
		subject := message.Subject
		if len(subject) > 200 {
			subject = subject[:200]
		}
		if message.From.EmailAddress.Address == "" {
			continue
		}
		err = state.attachInbound(r, "email", inboundMessage{ID: "graph:" + message.ID, OrgID: org, From: message.From.EmailAddress.Address, Subject: subject, Body: body, OccurredAt: message.ReceivedDateTime})
		if err != nil {
			break
		}
	}
	if err == nil {
		err = saveStoreDeltaTx(tx, before, state.snapshot(), changesBetween(before, state.snapshot(), "mailbox:graph"))
	}
	if err == nil {
		_, err = tx.Exec("INSERT INTO app_meta(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", "graph_cursor_"+org, next)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		tx.Rollback()
		state.restoreStore(before)
		setSyncStatus("failed", "Could not save mailbox messages; the page will be retried")
		return
	}
	status := "synced"
	if page.Next != "" {
		status = "syncing"
	}
	setSyncStatus(status, "")
}
